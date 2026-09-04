package message

// Part 消息部件标记：文本/艾特/markdown/图片/媒体都实现此接口。
// 可单独 Send，也可 Add 进 MsgBuilder 聚合。
type Part interface{ part() }
