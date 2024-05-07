package tools

import "sync"

// Set 集合运算
type Set struct {
	sync.RWMutex
	// 如果集合中的元素不是键值对，而只是单纯的字符串值，
	//也可以将 m 的类型定义为 map[string]bool，其中值的类型为 bool，表示集合中的元素是否存在。
	//这种方式可以更节省内存，因为不需要存储值，只需要存储键即可。
	m map[string]bool
}

func NewSet(items ...string) (s *Set) {
	s = &Set{
		m: make(map[string]bool, len(items)),
	}
	s.Add(items...)
	return s
}

func (s *Set) Add(items ...string) {
	s.Lock()
	defer s.Unlock()
	for _, v := range items {
		s.m[v] = true
	}
}

func (s *Set) Remove(items ...string) {
	s.Lock()
	defer s.Unlock()
	for _, v := range items {
		delete(s.m, v)
	}
}

func (s *Set) Contains(items ...string) bool {
	s.RLock()
	defer s.RUnlock()
	for _, v := range items {
		if _, ok := s.m[v]; !ok {
			return false
		}
	}
	return true
}

func (s *Set) Len() int {
	return len(s.m)
}

func (s *Set) ToList() []string {
	s.RLock()
	defer s.RUnlock()
	list := make([]string, 0, len(s.m))
	for item := range s.m {
		list = append(list, item)
	}
	return list
}

func (s *Set) Union(sets ...*Set) *Set {
	r := NewSet(s.ToList()...) // 相当于复制一遍set
	for _, set := range sets {
		for e := range set.m {
			r.m[e] = true
		}
	}
	return r
}

func (s *Set) Minus(sets ...*Set) *Set {
	r := NewSet(s.ToList()...)
	for _, set := range sets {
		for e := range set.m {
			if _, ok := s.m[e]; ok {
				delete(r.m, e)
			}
		}
	}
	return r
}

// Intersect 交集
func (s *Set) Intersect(sets ...*Set) *Set {
	r := NewSet(s.ToList()...)
	for _, set := range sets {
		for e := range s.m {
			if _, ok := set.m[e]; !ok {
				delete(r.m, e)
			}
		}
	}
	return r
}

// Complement 补集
func (s *Set) Complement(full *Set) *Set {
	r := NewSet()
	for e := range full.m {
		if _, ok := s.m[e]; !ok {
			r.Add(e)
		}
	}
	return r
}
