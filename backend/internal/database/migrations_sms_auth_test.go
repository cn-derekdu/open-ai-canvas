package database

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// smsAuthVerificationMigrationName 是补回短信验证对象的那个迁移，测试按名字定位它。
const smsAuthVerificationMigrationName = "sms_auth_verification"

// legacySMSAuthDatabase 复现线上故障现场：数据库已应用到 v35，但物理结构缺少
// 短信/手机号验证特性依赖的表与列（该特性只改了 schema.go 基线，未登记迁移）。
func legacySMSAuthDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := Open(Config{Driver: "sqlite", DSN: "file:" + t.Name() + "?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	// 应用除 v36 之外的全部迁移，随后抹掉短信验证特性引入的对象。
	// 按迁移名定位而不是"最后一条"：后续新增迁移（如 v37~v39）会让此前那个假设失效，
	// 把 v36 也应用掉，于是真正迁移时被记录为已完成，短信对象永远补不回来。
	for _, item := range schemaMigrations {
		if item.name == smsAuthVerificationMigrationName {
			continue
		}
		if err := item.apply(db); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("DROP INDEX IF EXISTS idx_users_phone_nonempty").Error; err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"phone", "phone_verified_at", "email_verified_at"} {
		if db.Migrator().HasColumn(&model.User{}, column) {
			if err := db.Migrator().DropColumn(&model.User{}, column); err != nil {
				t.Fatalf("模拟旧库时删除 users.%s 失败：%v", column, err)
			}
		}
	}
	for _, dropped := range []any{&model.SMSChannel{}, &model.SMSRecord{}, &model.AuthVerification{}, &model.NotificationQuota{}} {
		if err := db.Migrator().DropTable(dropped); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestMigrateSchemaV36RestoresSMSAuthObjects(t *testing.T) {
	db := legacySMSAuthDatabase(t)

	// 前置断言：确认现场确实缺对象，否则本测试无法证明修复有效。
	if db.Migrator().HasColumn(&model.User{}, "phone") || db.Migrator().HasTable(&model.SMSChannel{}) {
		t.Fatal("旧库仍具备短信验证对象，测试前置条件不成立")
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := MigrateSchema(db); err != nil {
			t.Fatal(err)
		}
	}

	for _, column := range []string{"phone", "phone_verified_at", "email_verified_at"} {
		if !db.Migrator().HasColumn(&model.User{}, column) {
			t.Fatalf("迁移后仍缺少 users.%s", column)
		}
	}
	for _, wanted := range []any{&model.SMSChannel{}, &model.SMSRecord{}, &model.AuthVerification{}, &model.NotificationQuota{}} {
		if !db.Migrator().HasTable(wanted) {
			t.Fatalf("迁移后仍缺少表 %T", wanted)
		}
	}
	if !db.Migrator().HasIndex(&model.User{}, "idx_users_phone_nonempty") {
		t.Fatal("迁移后仍缺少用户手机号唯一索引")
	}
	status, err := ReadSchemaStatus(db)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready || status.Current != CurrentSchemaVersion {
		t.Fatalf("迁移后结构状态异常：%#v", status)
	}
}

// TestMigrateSchemaV36UnblocksBrokenLoginQueries 直接验证故障期间失败的查询已可执行：
// 登录会写 users.phone，认证设置页会读 sms_channels。
func TestMigrateSchemaV36UnblocksBrokenLoginQueries(t *testing.T) {
	db := legacySMSAuthDatabase(t)
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&model.User{
		ID:       "user-login-probe",
		Username: "probe",
		Email:    "probe@example.com",
		Status:   model.UserStatusActive,
	}).Error; err != nil {
		t.Fatalf("创建探针用户失败：%v", err)
	}
	if err := db.Model(&model.User{}).Where("id = ?", "user-login-probe").
		Updates(map[string]any{"phone": "13800000000", "phone_verified_at": time.Now().UTC()}).Error; err != nil {
		t.Fatalf("登录写回手机号失败：%v", err)
	}

	var found model.User
	if err := db.First(&found, "phone = ?", "13800000000").Error; err != nil {
		t.Fatalf("按手机号查询用户失败：%v", err)
	}
	if found.Phone != "13800000000" {
		t.Fatalf("手机号未正确落库：%q", found.Phone)
	}

	var channels []model.SMSChannel
	if err := db.Order("priority ASC, id ASC").Find(&channels).Error; err != nil {
		t.Fatalf("读取短信渠道失败：%v", err)
	}
}
