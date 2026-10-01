package app

// 权限判定必须认真实画布存储（canvas_projects）。
//
// 历史实现读的是遗留的 canvases 表（model.Canvas）：那张表在本系统的新链路里从不写入，
// 于是每次 Agent 会话都会静默退化成"只读"的最小权限集，并在日志里刷一条
// `relation "canvas" does not exist` 的 WARN。本用例锁住"以 canvas_projects 判所有权"。

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestBuildPermissionsConfigUsesCanvasProjectOwnership(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	if err := db.Create(&model.CanvasProject{ID: "perm-canvas", UserID: "user", PayloadJSON: `{"nodes":[]}`}).Error; err != nil {
		t.Fatal(err)
	}

	owner := s.buildPermissionsConfig("user", "perm-canvas")
	for _, key := range []string{"canWriteCanvas", "canCreateNodes", "canMoveNodes", "canDuplicateNodes", "canManageRelations", "canDeleteNodes"} {
		if owner[key] != true {
			t.Fatalf("画布所有者应拿到 %s=true，实际：%#v", key, owner)
		}
	}
	if owner["canReadCanvas"] != true {
		t.Fatalf("所有者始终可读，实际：%#v", owner)
	}

	other := s.buildPermissionsConfig("other-user", "perm-canvas")
	for _, key := range []string{"canWriteCanvas", "canCreateNodes", "canMoveNodes", "canDuplicateNodes", "canManageRelations", "canDeleteNodes"} {
		if other[key] != false {
			t.Fatalf("非所有者必须保持只读默认 %s=false，实际：%#v", key, other)
		}
	}
	if other["canReadCanvas"] != true {
		t.Fatalf("只读默认仍应可读，实际：%#v", other)
	}

	missing := s.buildPermissionsConfig("user", "not-exist-canvas")
	if missing["canWriteCanvas"] != false || missing["canReadCanvas"] != true {
		t.Fatalf("画布不存在时应保持只读默认，实际：%#v", missing)
	}
}
