package database

import (
	"infinite-canvas/backend/internal/model"
	"testing"
)

func TestTaskMediaRecoveryMigrationPreservesHistoricalTasks(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE tasks (id TEXT PRIMARY KEY, status TEXT, error TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO tasks VALUES ('old-task', 'failed', 'download failed')`).Error; err != nil {
		t.Fatal(err)
	}
	// 本地二开在 v16 插入了独有迁移，编号相对上游永久偏移 +1，所以这里不能按下标取：
	// 上游 schemaMigrations[33] 是 task_media_recovery，本地同一下标落到
	// oauth_state_accepted_terms，会去改测试库里并不存在的 o_auth_states。
	// 按名称查找，避免每次上游新增迁移都要回来改这个下标。
	var recovery migration
	for _, item := range schemaMigrations {
		if item.name == "task_media_recovery" {
			recovery = item
			break
		}
	}
	if recovery.apply == nil {
		t.Fatal("未找到 task_media_recovery 迁移")
	}
	for i := 0; i < 2; i++ {
		if err := recovery.apply(db); err != nil {
			t.Fatal(err)
		}
	}
	var task model.Task
	if err := db.First(&task, "id = ?", "old-task").Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != model.TaskStatusFailed || task.Error != "download failed" || task.MediaRecoveryJSON != "" || task.MediaStage != "" {
		t.Fatalf("migration rewrote historical task: %+v", task)
	}
	for _, field := range []string{"media_stage", "media_recovery_json"} {
		if !db.Migrator().HasColumn(&model.Task{}, field) {
			t.Fatalf("missing %s", field)
		}
	}
}
