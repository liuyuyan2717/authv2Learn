package main

import (
    "context"
    "encoding/json"
    "time"

    oauth2 "github.com/go-oauth2/oauth2/v4"
    "github.com/go-oauth2/oauth2/v4/models"
    "github.com/go-redis/redis/v8"
    "github.com/google/uuid"
)

type MyRedisTokenStore struct {
    client *redis.Client
    prefix string
}

func NewMyRedisTokenStore(cli *redis.Client, prefix string) *MyRedisTokenStore {
    return &MyRedisTokenStore{client: cli, prefix: prefix}
}

func (s *MyRedisTokenStore) wrapperKey(parts ...string) string {
    // prefix: myToken
    // keys example: myToken:access:<token>, myToken:refresh:<token>, myToken:basic:<basicID>
    key := s.prefix
    for _, p := range parts {
        key += ":" + p
    }
    return key
}

func (s *MyRedisTokenStore) Create(ctx context.Context, info oauth2.TokenInfo) error {
    now := time.Now()
    ct := info.GetAccessCreateAt()
    if ct.IsZero() {
        ct = now
    }

    // serialize token info
    mt := &models.Token{
        ClientID:        info.GetClientID(),
        UserID:          info.GetUserID(),
        RedirectURI:     info.GetRedirectURI(),
        Scope:           info.GetScope(),
        Code:            info.GetCode(),
        CodeCreateAt:    info.GetCodeCreateAt(),
        CodeExpiresIn:   info.GetCodeExpiresIn(),
        Access:          info.GetAccess(),
        AccessCreateAt:  info.GetAccessCreateAt(),
        AccessExpiresIn: info.GetAccessExpiresIn(),
        Refresh:         info.GetRefresh(),
        RefreshCreateAt: info.GetRefreshCreateAt(),
        RefreshExpiresIn: info.GetRefreshExpiresIn(),
    }
    jv, err := json.Marshal(mt)
    if err != nil {
        return err
    }

    basicID := uuid.Must(uuid.NewRandom()).String()
    aexp := info.GetAccessExpiresIn()
    rexp := aexp

    if refresh := info.GetRefresh(); refresh != "" {
        rexp = info.GetRefreshCreateAt().Add(info.GetRefreshExpiresIn()).Sub(ct)
        if aexp.Seconds() > rexp.Seconds() {
            aexp = rexp
        }
    }

    pipe := s.client.TxPipeline()

    if refresh := info.GetRefresh(); refresh != "" {
        pipe.SetEX(ctx, s.wrapperKey("refresh", refresh), basicID, rexp)
    }
    if access := info.GetAccess(); access != "" {
        pipe.SetEX(ctx, s.wrapperKey("access", access), basicID, aexp)
    }
    // details live with refresh lifetime (or access if no refresh)
    pipe.SetEX(ctx, s.wrapperKey("basic", basicID), string(jv), rexp)

    _, err = pipe.Exec(ctx)
    return err
}

func (s *MyRedisTokenStore) RemoveByCode(ctx context.Context, code string) error {
    // not used in password/client credentials usually; safe no-op
    return nil
}

func (s *MyRedisTokenStore) RemoveByAccess(ctx context.Context, access string) error {
    // get basic id first to remove detail if needed
    ctx = contextWithTimeout(ctx)
    bid, err := s.client.Get(ctx, s.wrapperKey("access", access)).Result()
    if err == nil {
        // delete mapping; keep basic until refresh expires
        s.client.Del(ctx, s.wrapperKey("access", access))
        _ = bid
        return nil
    }
    if err == redis.Nil {
        return nil
    }
    return err
}

func (s *MyRedisTokenStore) RemoveByRefresh(ctx context.Context, refresh string) error {
    ctx = contextWithTimeout(ctx)
    bid, err := s.client.Get(ctx, s.wrapperKey("refresh", refresh)).Result()
    if err == nil {
        s.client.Del(ctx, s.wrapperKey("refresh", refresh))
        // also remove detail
        s.client.Del(ctx, s.wrapperKey("basic", bid))
        return nil
    }
    if err == redis.Nil {
        return nil
    }
    return err
}

func (s *MyRedisTokenStore) GetByCode(ctx context.Context, code string) (oauth2.TokenInfo, error) {
    // not used in password/client credentials
    return nil, redis.Nil
}

func (s *MyRedisTokenStore) GetByAccess(ctx context.Context, access string) (oauth2.TokenInfo, error) {
    ctx = contextWithTimeout(ctx)
    bid, err := s.client.Get(ctx, s.wrapperKey("access", access)).Result()
    if err != nil {
        return nil, err
    }
    raw, err := s.client.Get(ctx, s.wrapperKey("basic", bid)).Result()
    if err != nil {
        return nil, err
    }
    var mt models.Token
    if err := json.Unmarshal([]byte(raw), &mt); err != nil {
        return nil, err
    }
    return &mt, nil
}

func (s *MyRedisTokenStore) GetByRefresh(ctx context.Context, refresh string) (oauth2.TokenInfo, error) {
    ctx = contextWithTimeout(ctx)
    bid, err := s.client.Get(ctx, s.wrapperKey("refresh", refresh)).Result()
    if err != nil {
        return nil, err
    }
    raw, err := s.client.Get(ctx, s.wrapperKey("basic", bid)).Result()
    if err != nil {
        return nil, err
    }
    var mt models.Token
    if err := json.Unmarshal([]byte(raw), &mt); err != nil {
        return nil, err
    }
    return &mt, nil
}

// small helper for bounded redis ops
func contextWithTimeout(ctx context.Context) context.Context {
    if ctx == nil {
        ctx = context.Background()
    }
    c, _ := context.WithTimeout(ctx, 3*time.Second)
    return c
}


