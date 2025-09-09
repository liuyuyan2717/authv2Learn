# OAuth2 服务 (go-oauth2 + Redis)

- 语言: Go
- 存储: 本地 Redis `127.0.0.1:6379`
- 接口: `/token` 获取访问令牌, `/validate` 验证令牌

## 环境要求
- Go 1.20+
- 本地 Redis 已启动: `127.0.0.1:6379`

## 安装依赖
```bash
# 在项目根目录
go mod tidy
```

## 运行
```bash
go run .
# 服务器监听 :8080
```

## 获取 Token
- 密码模式(password): 预置用户 `user/pass`
- 预置客户端: `client_id=client_1` `client_secret=secret_1`

```bash
curl -s -X POST \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=client_1&client_secret=secret_1&username=user&password=pass" \
  http://127.0.0.1:8080/token | jq .
```

也可使用 `client_credentials` 模式：
```bash
curl -s -X POST \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&client_id=client_1&client_secret=secret_1" \
  http://127.0.0.1:8080/token | jq .
```

返回示例:
```json
{
  "access_token": "<token>",
  "token_type": "Bearer",
  "expires_in": 7200
}
```

## 验证 Token
```bash
ACCESS_TOKEN="<token>"
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" http://127.0.0.1:8080/validate | jq .
```

返回示例:
```json
{"active":true,"client_id":"client_1","user_id":"user_1"}
```

## 说明
- 令牌存储在 Redis 中。
- 客户端与用户校验逻辑为演示用途，可替换为数据库或企业身份源。

