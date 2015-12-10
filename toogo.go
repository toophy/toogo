package toogo

// toogo game server framework version.
const VERSION = "0.0.1"

// 唯一ID架构
// pid 占有最小10位 (平台表规定)
// aid 占有pid上面的32位, (自增)
// sid 占有aid上面的14位, (服务器小区号,开服参数为准)
// 剩余最上面8位, 预留
// 以下是合成的跨平台唯一ID
// Aid 帐号ID
// 2^8 | 2^14 | 2^32 | 2^10
//  0     0      aid    pid
// Sid 小区ID
// 2^8 | 2^14 | 2^32 | 2^10
//  0    sid    0      pid
// Pid 平台ID
// 2^8 | 2^14 | 2^32 | 2^10
//  0    0      0      pid
// Rid 角色ID, 自定义为一个小区一个角色, 如果有多个角色, 可以启用预留的8位
// 2^8 | 2^14 | 2^32 | 2^10
//  0    sid    aid    pid
//
//
