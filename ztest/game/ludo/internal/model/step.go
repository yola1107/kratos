package model

// Step 代表一次移动的步骤结果
type Step struct {
	Id     int32         // 棋子ID
	X      int32         // 移动步数
	From   int32         // 起始位置
	To     int32         // 目标位置
	Color  int32         // 棋子颜色
	Killed []*KilledInfo // 击杀信息
}

type KilledInfo struct {
	Id    int32
	From  int32
	To    int32
	Color int32
}

func (s *Step) Clone() *Step {
	if s == nil {
		return nil
	}
	cp := *s
	cp.Killed = make([]*KilledInfo, len(s.Killed))
	for i, k := range s.Killed {
		cp.Killed[i] = &KilledInfo{Id: k.Id, From: k.From, To: k.To, Color: k.Color}
	}
	return &cp
}
