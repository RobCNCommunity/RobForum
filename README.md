# Roblox Community

独立的 Roblox 中文玩家社区，后端使用 Go + Chi + MariaDB/MySQL，前端使用 Vue 3 + TypeScript + NutUI。

## 当前已实现

- MySQL/MariaDB 生产存储和嵌入式迁移
- 邮箱/密码注册和登录，HttpOnly 会话 Cookie
- SMTP 配置、加密密码保存、测试邮件
- 忘记密码和重置密码邮件流程
- 可选注册邮箱验证码，带冷却、过期和尝试次数限制
- GT4、GT3、阿里云验证码配置预留和后台管理
- 社区名称、Logo、头像、主色、域名和注册策略配置
- 首页动态、板块、搜索、帖子、评论和发帖
- 帖子多图上传、时间线缩略图和详情页缩放预览
- 关注时间线、点赞、转发、收藏和互动通知
- 私信、群聊、用户搜索、热门用户和拉黑
- 个人资料编辑、头像上传、简介和公开个人主页
- 用户动态与已审核资源聚合展示，作者蓝微标识同步显示
- 蓝微认证申请、管理员审核、认证名称和审计记录
- 资源投稿、100 MB 上传限制、扩展名/MIME/可执行文件检测和 SHA-256
- 用户投稿记录、管理员审核、拒绝、下架和审核流水
- 可配置“发帖先审后发”，普通用户新帖默认进入人工审核队列
- 管理员内容审核：通过、驳回、隐藏和不可恢复删除帖子
- 管理员用户治理：搜索、封禁、解封和匿名化软删除账号
- 只有审核通过的资源才能公开展示和下载
- EPay 兼容支付配置、订单创建、签名回调、金额校验和重复回调幂等处理
- 付费资源销售分成、创作者余额和管理员审核提现
- Roblox 身份绑定挑战流程接口预留
- 桌面端侧边导航和手机端抽屉导航

当前资源文件保存在受控本地目录，后续可以通过存储接口切换到 S3 兼容对象存储。会员、广告位和活动仍在后续阶段接入。

## 支付配置

管理员在“管理后台 → 支付网关”填写 EPay 兼容网关地址、商户号、密钥和支付类型。网关地址填写站点根地址即可，系统会自动请求其 `/submit.php`。

- 异步通知默认地址：`https://你的域名/api/v1/payment/callback`
- 支付完成后，网关必须向异步通知地址发送 `sign`、`trade_status`、`out_trade_no`、`money` 等参数
- 系统会校验 MD5 签名、订单金额、资源当前售价和订单状态
- 创作者收入按订单金额的 90% 写入不可变流水，平台保留 10% 服务费
- 提现申请最低 10 元；管理员确认线下转账后标记“已打款”，驳回会释放冻结余额

支付回调只负责确认订单和发放下载权限，不会自动向创作者第三方账户转账。上线前请先使用网关沙箱完成一笔成功、金额不一致、重复回调和签名错误测试。

## 本地运行

先准备 MySQL 8/MariaDB，并创建数据库与账号。也可以使用：

```bash
docker compose up -d mysql
```

然后设置环境变量：

```bash
ROBLOX_DB_DRIVER=mysql
ROBLOX_MYSQL_DSN='roblox:change-me@tcp(127.0.0.1:3306)/roblox_community?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=UTC'
ROBLOX_MASTER_KEY='replace-with-a-long-random-secret'
ROBLOX_ADMIN_EMAIL='admin@example.com'
ROBLOX_ADMIN_PASSWORD='replace-with-a-unique-password'
```

启动后端：

```bash
go run ./cmd/server
```

启动前端：

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

## 生产部署

生产环境使用 `/opt/roblox-community/bin/roblox-community` 和 `/opt/roblox-community/frontend/dist`，将 `deploy/roblox-community.service` 安装到 systemd 后，通过 Nginx 反向代理到 `127.0.0.1:8088`。

`ROBLOX_MASTER_KEY` 不能更换，否则已加密的 SMTP 和验证码密钥无法解密。上线前必须设置唯一的管理员密码、数据库密码和公网地址。
