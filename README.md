## Polarix 北极星

> QQ官方机器人 轻量开发框架
>
> Without AI feature

### 配置文件
在根目录下新建`config.json`, 并按照下方格式填写
```json
{
  "port": 8080,
  "appid": "10000",
  "secret": "11111",
  "proxy": "https://api.sgroup.qq.com"
}
```

额外参数:

- uin: 机器人QQ号
- uid: 不知道是哪个UID, 可以在196173384群里发送`转[机器人QQ号]`查询, 其他方法正在寻找, 后续跟进

(这两个参数可以不填写, 并非必须参数)

#### 配置文件说明
- port    服务端口
- appid   机器人ID
- secret  机器人AppSecret
- proxy   代理地址

> 什么是代理地址?
> 
> 代理地址是为了QQ开放平台IP白名单限制所使用的功能, 当你的服务器处于动态IP的时候, 可以在一个固定IP的设备上搭建反代服务, 然后填写对应的地址

### WebHook配置
在QQ开放平台里配置, WebHook填写`你的地址:端口/webhook`

事件按照你的需求勾选, 也可以一次性全部选择

***

### 组织方式
框架使用`插件`的形式来增加功能

### 插件管理面板

启动服务后访问 `/admin` 查看插件目录，每个插件进入独立的 `/admin/plugins/{id}` 配置页面。页面支持跟随系统、浅色和深色三种主题，选择会保存在浏览器中。插件配置保存到 `config.json` 的 `plugin_settings` 字段并即时生效。生图插件可在此配置 OpenAI 兼容接口，用户通过 `/draw <图片描述>` 生成图片；发送指令时附带一张或多张图片会自动调用 `/images/edits`，将附件作为参考图，最多使用 16 张。

未设置 `admin_password` 时，管理页面仅允许从服务器本机访问。需要远程管理时，在 `config.json` 中增加管理密码：

```json
{
  "admin_password": "请设置高强度密码"
}
```

远程访问时使用 HTTP Basic Auth，用户名固定为 `admin`，密码为 `admin_password`。生产环境应通过 HTTPS 反向代理访问管理页面。

插件通过 `Plugin.Config` 声明配置项，不需要自行实现管理页面或 HTTP 接口。`password` 字段只向面板返回是否已配置，留空保存时会保留原值：

```go
plugin.Register(&plugin.Plugin{
    Id:          "example",
    Name:        "示例插件",
    Description: "插件配置示例",
    Config: []plugin.ConfigField{
        {Key: "enabled", Label: "启用插件", Type: "boolean"},
        {Key: "endpoint", Label: "接口地址", Type: "text"},
        {Key: "api_key", Label: "API Key", Type: "password"},
    },
    ValidateConfig: validateConfig,
    ApplyConfig:    applyConfig,
})
```

`ValidateConfig` 在写入配置前执行，`ApplyConfig` 在启动加载和面板保存后执行。

指令还可以声明生命周期钩子：

```go
&plugin.Command{
    Prefix: "/example",
    Handle: handle,
    PermissionDenied: func(ctx *context.MessageContext) error {
        return ctx.Text("你无权使用此指令").Send()
    },
    HandleError: func(ctx *context.MessageContext, commandErr error) error {
        return ctx.Text("指令执行失败").Send()
    },
}
```

`PermissionDenied` 会在角色权限、私聊限制或黑白名单拒绝时调用。`HandleError` 会在 `Handle` 返回非空错误或发生 panic 后调用；原始错误及错误处理函数自身的错误仍会写入日志。子指令使用实际命中的子指令钩子。

管理面板也会为所有插件自动提供访问控制，无需插件额外声明。可以设置插件默认规则，并为具体指令或子指令覆盖：

- `关闭限制`：所有用户和群均可使用。
- `白名单`：用户 OpenID 或群 OpenID 命中任一名单时允许使用。
- `黑名单`：用户 OpenID 或群 OpenID 命中任一名单时拒绝使用。
- 指令选择 `继承插件规则` 时使用插件默认规则。

访问控制保存在 `config.json` 的 `plugin_access` 字段。群聊优先使用发送者的 `member_openid`，缺失时使用 `union_openid`；私聊使用 `user_openid`。被拒绝的指令会静默忽略。

#### 新建插件
在`plugins`(注意不是`lib/plugin`)目录下新建一个文件夹, 然后放入你的插件代码

如下是一个插件模板
```go
package echo

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"Plrx/lib/structers"
)

func init() {
	var commands []*plugin.Command
	commands = append(commands, &plugin.Command{
		Prefix:    "/echo",
		Role:      constant.RoleMember,
		Describle: "回显",
		Handle:    echoHandle,
	})

	self := plugin.PluginConfig{
		Id:       "echo",
		Commands: commands,
	}
	plugin.Register(&self)
}

func echoHandle(ctx *context.Context) error {
	return ctx.Client.SendGroupMessage(*ctx.Message, ctx.Message.GroupId)
}
```

#### 插件元信息

- Id          插件ID, 用于日志排查
- Commands    指令列表, 用于注册指令

#### 新建指令
一个指令需要`前缀` / `使用权限` / `描述`(暂无功能) / `处理函数`

并且可以额外添加`解析器`及`解析模板`

##### 前缀
> Prefix
指令前缀, 只有以该前缀开头的指令会传入插件

根据注册顺序, 后注册的插件如果跟之前注册插件的前缀相同, 会发生**覆盖**

##### 使用权限
> Role | 枚举值: **constant.RoleMember** | **constant.RoleAdmin** | **constant.RoleOwner**
最低使用指令的成员身份, 依次为**普通成员**、**管理员**和**群主**

不满足身份要求会静默失败

##### 处理函数
> Handle | type HandleFunc func(*context.Context) error

其中`*context.Context`为上下文对象, 其API用法见后文

函数需要返回一个`error`, 会显示在日志里, 不会发送到QQ里

##### 解析器&解析模板
> Parser & ParserTarget
> 
> 两者必须合用, 否则可能引发panic或预期之外的行为

解析器接受一个`Parser`接口, 其需要一个`Parse(rawMsg string, result any) error`函数, 该函数接收**原始消息**及**接收者指针**并返回一个`error`

- 当`Parser`没有被指定时, 默认使用`DefaultParser`(lib/parser/default.go), 除此之外还提供一个`PositionalParser`解析器

`DefaultParser`会将**原始消息**直接传给**接收者**, 不做任何处理

`PositionalParser`必须和`ParserTarget`配合使用, 会将指令参数解析到结构体里

- 当`ParserTarget`没有指定时, 默认使用`string`类型

当解析器为`PositionalParser`, 必须指定`ParserTarget`为一个从**结构体**构造的`reflect.Type`对象(`reflect.TypeOf`), 可以参考**ping**插件


#### 注册插件

在`plugins/register.go`中**匿名导入**你的插件所在的包

```go
import	_ "Plrx/plugins/ping"

```

***

### 上下文对象

这里假设传入的Context被`ctx`变量接收

#### 发送消息
有两种方式, 一种是手动构造`Message`对象, 而更推荐的是调用`Reply`快捷函数

##### Reply函数

直接调用传入的`ctx`的`Reply`函数:

```go
ctx.Reply("", structers.PlainText)
```

第一个参数是**消息内容**, 第二个参数是**消息类型**

**消息类型**为`MessageType`枚举, 可以选择`PlainText`及`Markdown`两种类型, 第一个为`纯文本`, 第二个为`Markdown`

**消息内容**在消息类型为`Markdown`的时候, 会被渲染为Markdown

##### Message对象
> 该对象的定义位于**lib/structers/msg.go**

1. 构造Message对象

你至少需要定义如下内容:

- Content
- MessageType

参数的含义与Reply中的一致

2. 发送消息

调用`ctx.Client.SendGroupMessage`或`ctx.Client.SendPrivateMessage`(根据发送目标)

当调用`ctx.Client.SendGroupMessage`时, 默认为**主动推送**消息, 如果需要采取**被动回复**(这两者区别见QQ官方文档), 需要在`Message`结构体中填入`MessageId`参数, 指定回复消息的ID

两个函数的**第一个参数**均为`Message`对象, 第二个参数分别为**群OpenID**及**用户ID**, 其中群OpenID的查询可以使用`echo`插件下的`/groupid`指令

#### 消息内容

##### 原始消息

位于`ctx.Message.Content`

##### 解析器产物

位于`ctx.Parserd`, 必须进行**类型断言**

##### 消息ID

位于`ctx.Message.MessageId`

##### 消息对象

位于`ctx.Message`

#### 发送者信息

包含在`ctx.Message`中, 为其下的`UserId`、`UnionId`及`GroupId`字段

#### 消息来源

目前仅存在两种枚举: `PrivateMessage`及`GroupMessage`

#### 公共请求对象

插件的请求应该调用上下文中的`Requests`, 该对象目前支持两种请求方式: `ctx.Requests.Get`及`ctx.Requests.Post`

##### Get请求

参数: `Get(url string, result any, headers map[string]string)`

- url 请求目标
- result 返回结果绑定目标(需要包含json标签的结构体), nil时不解析
- headers 请求头

##### Post请求

参数: `Post(url string, body any, result any, headers map[string]string)`

- url 请求目标
- body 请求体(可以为`[]byte`或者为可以被`json.Marshal`的对象)
- result/headers 同上

***

### Markdown模板

可以在`templates/markdown`下面存放多个`.md`文件, 每个文件为一个Markdown模板, 非`.md`文件会被忽略

在Markdown模板里, 可以使用插值语法":
```markdown
## {{ aaa }}
```

文件名(**不包含**.md后缀)将作为模板ID

通过调用`lib/templates`的`FillMarkdownTemplate(Id string, args Args)`函数可以填充模板

该函数需要两个参数

- Id 模板ID
- args 参数列表

#### 参数列表

是由`type Args map[string]any`定义的, 可以通过类似于:
```go
templates.Args{
	"name":        data.Data.Name,
	"look":        data.Data.Look,
}
```
的方式直接声明, 原本的`map[string]string`不再使用

参数既可以是`string`也可以是`int, int64, float64`

#### 可能的错误

当模板ID不存在时, 返回错误

当参数列表args传入的参数不满足模板里定义的**所有**插值时, 返回错误

当参数列表args传入的结构体中有无法使用的类型时, 返回错误

#### 追加图片元信息

QQ的Markdown无法自适应图片大小, 必须追加元信息才能正常显示:
```markdown
![alt #300px #400px](https://aaa.com/bbb.jpg)
```
可以调用`ProcessMarkdownImages`辅助函数, 该函数会自动处理所有图片引用并追加元信息

***

### 按钮

代码位于`lib/structers/buttons/buttons.go`, 示范在`echo`插件的`/uid`指令

一个消息可以附带一个`Keyboard`, 一个`Keyboard`最多五行, 每行最多五个按钮, 共25个

#### 创建按钮

通过`&buttons.Keyboard{}`初始化一个变量(假设为`keyboard`), 作为承载按钮的变量

然后调用`keyboard.AppendButton`, 如下

```go
button, err := keyboard.AppendButton("ID", "点击前文本", "点击后文本", ButtonStyle.Blue, 0)
```

- `"ID"` 按钮ID, 在一个Keyboard内必须唯一
- `"点击前文本"` & `"点击后文本"` 不予解释
- `ButtonStyle.Blue` 按钮边框样式, 是`lib/constant/Button/ButtonStyle.go`下的枚举, 只支持`Blue`和`Gray`
- `0` 在哪一行追加按钮, 从**0**开始, 最大为**4**

需要判断`err`是否为`nil`

`button`为`*Button`类型, 是修改按钮的指针, 不得进行值拷贝, 否则修改操作会失效

#### 设置按钮行为

调用`button`的函数

- SetAutoCommand 设置自动发送消息, 参数依次为: 消息内容、是否自动发送(仅私聊有效)、是否拉起图片选择(仅手机端有效, 目前无法使用, 请保持`false`)
- SetHref 设置跳转链接, 参数为: 链接地址, 需要携带协议头
- SetCallback 设置回调, 参数为: 回调数据(当前框架没有处理事件回调, 后续会进行补充)

#### 设置按钮权限

调用`button.SetPermission`函数, 传入一个`lib/constant/Button/ActionPermissionType`下的枚举, 注意这个函数只应该传入`Admin`(仅管理员可用)或者`AllUser`(所有人可用)

当需要设置部分用户可用时, 需要使用`button.SetUserWhiteList`, 并传入一个`[]string`作为允许使用的用户的*OpenID*

#### 设置其他内容

##### 不支持按钮的情况

调用`button.SetUnsupportedTip`设置不支持按钮的时候的提示文本

***

### TODO

- [x] 支持按钮功能
- [ ] 数据库API
- [ ] 按钮回调事件

#### 不会支持的功能
- 所有与频道相关的功能

## 许可证

MIT
