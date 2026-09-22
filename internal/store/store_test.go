package store

import (
	"errors"
	"testing"
)

func TestUpdateRollbackAndNormalize(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Update(func(d *Data) error {
		d.Groups = []Group{{ID: "b", Order: 5}, {ID: "a", Order: 2}}
		d.Items = []Item{{ID: "1", GroupID: "a", Order: 9}, {ID: "2", GroupID: "a", Order: 3}, {ID: "x", GroupID: "gone"}}
		return nil
	})
	s.View(func(d *Data) {
		if d.Groups[0].ID != "a" || d.Groups[0].Order != 0 || d.Groups[1].Order != 1 {
			t.Fatalf("分组排序错误: %+v", d.Groups)
		}
		if len(d.Items) != 2 || d.Items[0].ID != "2" || d.Items[0].Order != 0 {
			t.Fatalf("卡片未正确排序或未清理孤立卡片: %+v", d.Items)
		}
	})
	err = s.Update(func(d *Data) error {
		d.Groups = nil
		return errors.New("失败")
	})
	if err == nil {
		t.Fatal("应返回错误")
	}
	s.View(func(d *Data) {
		if len(d.Groups) != 2 {
			t.Fatal("出错后应回滚修改")
		}
	})
	// 重新打开后数据应持久化
	s2, err := Open(s.path[:len(s.path)-len("/data.json")])
	if err != nil {
		t.Fatal(err)
	}
	s2.View(func(d *Data) {
		if len(d.Items) != 2 || d.Secret == "" {
			t.Fatal("数据未持久化")
		}
	})
}
