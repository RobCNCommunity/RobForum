# Roblox Community

独立的 Roblox 中文玩家社区，后端使用 Go + Chi + MariaDB/MySQL，前端使用 Vue 3 + TypeScript + NutUI。

## 目录与构建

- `frontend/`：唯一的 Vue 3 + Vite + NutUI 前端源码和构建产物来源。
- `cmd/`、`internal/`：Go 后端服务源码。
- `migrations/`：MySQL/MariaDB 数据库迁移。
- `web/`：空的历史目录，不是当前前端；可以忽略。

前端不使用 Python 构建。开发和生产构建均由 Vite 的 npm 脚本完成：

```bash
# 在项目根目录
npm run build

# 或在前端目录直接使用 Vite 脚本
cd frontend
npm run build
```

带类型检查的生产检查可使用 `npm run build:checked`。部署只上传 `frontend/dist/`，不要运行任何 Python 页面生成脚本。

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

内容审核在服务器环境变量中配置，密钥不可写入前端或仓库：

```bash
ROBLOX_CONTENT_MODERATION_ENABLED=true
ROBLOX_CONTENT_MODERATION_URL='https://provider.example'
ROBLOX_CONTENT_MODERATION_API_KEY='server-only-secret'
ROBLOX_CONTENT_MODERATION_MODEL='grok-4.5'
ROBLOX_CONTENT_MODERATION_TIMEOUT_SECONDS=30

# 可选：百度文本审核第二层。启用后必须同时配置真正的 API Key（AK）和
# Secret Key（SK）；AppID 不能替代 API Key。百度和上游审核均通过后才会放行。
ROBLOX_BAIDU_CONTENT_MODERATION_ENABLED=false
ROBLOX_BAIDU_CONTENT_MODERATION_API_KEY='baidu-api-key'
ROBLOX_BAIDU_CONTENT_MODERATION_SECRET_KEY='baidu-secret-key'
ROBLOX_BAIDU_CONTENT_MODERATION_APP_ID='optional-app-id'
ROBLOX_BAIDU_CONTENT_MODERATION_AUTH_URL='https://aip.baidubce.com/oauth/2.0/token'
ROBLOX_BAIDU_CONTENT_MODERATION_URL='https://aip.baidubce.com/rest/2.0/solution/v1/text_censor/v2/user_defined'
ROBLOX_BAIDU_CONTENT_MODERATION_STRATEGY_ID='optional-strategy-id'
ROBLOX_BAIDU_IMAGE_MODERATION_ENABLED=true
ROBLOX_BAIDU_IMAGE_MODERATION_URL='https://aip.baidubce.com/rest/2.0/solution/v1/img_censor/v2/user_defined'
ROBLOX_BAIDU_IMAGE_MODERATION_STRATEGY_ID='optional-image-strategy-id'
```

启用后，帖子标题/正文、评论、资料、认证申请、群名和私信/群聊消息均在写入数据库前进行文本审核；帖子图片、评论图片、头像、封面、资源预览图以及图片类型的资源文件会进行百度图像审核。资源标题和介绍仍进入人工审核队列。未显式设置 `ROBLOX_BAIDU_IMAGE_MODERATION_ENABLED` 时，图片审核开关继承百度文本审核开关。启用多个文本审核服务时，所有服务都必须通过；帖子只有在全部机审通过后才直接发布，机审不通过、服务异常、输出格式错误或审计记录失败时转入人工审核，其他内容仍拒绝提交。百度返回“不合规”和“疑似”都会拦截，审核失败按不可用处理；审计表只保存内容或图片的 SHA-256、审核结果和时间，不保存原文或图片。百度凭据只能放在服务器环境变量中，不能写入前端或仓库。
