package toogo

const (
	TgidType_Base      = 0       // 基本类型, Pid,Sid,Aid,Rid等
	TgidType_Monster   = 1       // 怪物类型, Mid
	TgidType_Guild     = 2       // 帮会类型, 大帮会
	TgidType_Base_R    = 0       // 基本类型优化后
	TgidType_Monster_R = 1 << 58 // 怪物类型优化后
	TgidType_Guild_R   = 2 << 58 // 帮会类型优化后
)

// type : 0 => Rid
// type : 1 => Mid
//
//
// 2^6 | 2^32 | 2^16 | 2^10
// type| aid  | sid  | pid
func Tgid_get_aid(id uint64) uint64 {
	return (id & 0x3FFFFFFFC000000) >> 26
}

func Tgid_get_pid(id uint64) uint64 {
	return (id & 0x3FF)
}

func Tgid_get_sid(id uint64) uint64 {
	return (id & 0x3FFFC00) >> 10
}

func Tgid_get_gid(id uint64) uint64 {
	return (id & 0x3FFFFFFFC000000) >> 26
}

func Tgid_get_type(id uint64) uint64 {
	return (id & 0xFC00000000000000) >> 58
}

// check type and sid => 0
// id & 0xFC00000003FFFC00 必须是 0
// pid > 0
// aid > 0
func Tgid_is_Aid(id uint64) bool {
	return id&0xFC00000003FFFC00 == 0 && id&0x3FF > 0 && id > 0x4000000
}

// check type and aid => 0
// pid > 0
// sid > 0
func Tgid_is_Sid(id uint64) bool {
	return id&0x3FF > 0 && id > 0x401 && id <= 0x3FFFFFF
}

func Tgid_is_Pid(id uint64) bool {
	return id > 0 && id <= 0x3FF
}

// check type => 0
// pid > 0
// aid > 0
// sid > 0
func Tgid_is_Rid(id uint64) bool {
	return id&0xFC00000000000000 == TgidType_Base_R && id&0x3FF > 0 && id&0x3FFFC00 > 0x3FF && id&0x3FFFFFFFC000000 > 0x3FFFFFF
}

func Tgid_is_Mid(id uint64) bool {
	return id&0xFC00000000000000 == TgidType_Monster_R && id&0x3FF > 0 && id&0x3FFFC00 > 0x3FF && id&0x3FFFFFFFC000000 > 0x3FFFFFF
}

func Tgid_is_Gid(id uint64) bool {
	return id&0xFC00000000000000 == TgidType_Guild_R && id&0x3FF > 0 && id&0x3FFFC00 > 0x3FF && id&0x3FFFFFFFC000000 > 0x3FFFFFF
}

func Tgid_make_Aid(aid, pid uint64) uint64 {
	return aid<<26 | pid
}

func Tgid_make_Sid(sid, pid uint64) uint64 {
	return sid<<10 | pid
}

func Tgid_make_Rid(aid, sid, pid uint64) uint64 {
	return aid<<26 | sid<<10 | pid
}

func Tgid_make_Gid(gid, sid, pid uint64) uint64 {
	return TgidType_Guild_R | gid<<26 | sid<<10 | pid
}
