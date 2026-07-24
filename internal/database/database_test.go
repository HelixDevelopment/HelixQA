package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPostgresDBStruct(t *testing.T) {
	db := &PostgresDB{}
	if db.Pool != nil {
		t.Error("expected nil Pool for zero-value PostgresDB")
	}
}

func TestRedisDBStruct(t *testing.T) {
	r := &RedisDB{}
	if r.Client != nil {
		t.Error("expected nil Client for zero-value RedisDB")
	}
}

func TestNewPostgres_InvalidURL(t *testing.T) {
	_, err := NewPostgres("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
	if !strings.Contains(err.Error(), "unable to parse database URL") {
		t.Errorf("expected error message to contain 'unable to parse database URL', got: %v", err)
	}
}

func TestNewPostgres_EmptyURL(t *testing.T) {
	_, err := NewPostgres("")
	if err == nil {
		t.Fatal("expected error for empty database URL")
	}
}

func TestNewPostgres_GarbageProtocol(t *testing.T) {
	_, err := NewPostgres("garbage://host:5432/db")
	if err == nil {
		t.Fatal("expected error for garbage protocol URL")
	}
	if !strings.Contains(err.Error(), "unable to parse database URL") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestNewRedis_InvalidURL(t *testing.T) {
	_, err := NewRedis("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid redis URL")
	}
	if !strings.Contains(err.Error(), "unable to parse redis URL") {
		t.Errorf("expected error message to contain 'unable to parse redis URL', got: %v", err)
	}
}

func TestNewRedis_EmptyURL(t *testing.T) {
	_, err := NewRedis("")
	if err == nil {
		t.Fatal("expected error for empty redis URL")
	}
}

func TestNewRedis_GarbageProtocol(t *testing.T) {
	_, err := NewRedis("garbage://host:6379")
	if err == nil {
		t.Fatal("expected error for garbage protocol URL")
	}
	if !strings.Contains(err.Error(), "unable to parse redis URL") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestNewRedis_UnreachableServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping unreachable server test in short mode")
	}

	_, err := NewRedis("redis://localhost:99999")
	if err == nil {
		t.Fatal("expected error for unreachable redis server")
	}
	if !strings.Contains(err.Error(), "unable to ping redis") {
		t.Errorf("expected error message to contain 'unable to ping redis', got: %v", err)
	}
}

func TestNewRedis_UnreachableServer_TLS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping unreachable server test in short mode")
	}

	_, err := NewRedis("rediss://localhost:99999")
	if err == nil {
		t.Fatal("expected error for unreachable TLS redis server")
	}
}

func TestNewPostgres_UnreachableServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping unreachable server test in short mode")
	}

	_, err := NewPostgres("postgres://user:pass@localhost:99999/dbname?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for unreachable postgres server")
	}
}

func TestPostgresDB_HealthCheck_NilPool(t *testing.T) {
	db := &PostgresDB{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Pool")
		}
	}()
	_ = db.HealthCheck(nil)
}

func TestPostgresDB_HealthCheck_CancelledContext(t *testing.T) {
	db := &PostgresDB{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Pool and cancelled context")
		}
	}()
	_ = db.HealthCheck(ctx)
}

func TestRedisDB_HealthCheck_NilClient(t *testing.T) {
	r := &RedisDB{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Client")
		}
	}()
	_ = r.HealthCheck(nil)
}

func TestRedisDB_HealthCheck_CancelledContext(t *testing.T) {
	r := &RedisDB{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Client and cancelled context")
		}
	}()
	_ = r.HealthCheck(ctx)
}

func TestPostgresDB_Close_NilPool(t *testing.T) {
	db := &PostgresDB{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling Close with nil Pool")
		}
	}()
	db.Close()
}

func TestRedisDB_Close_NilClient(t *testing.T) {
	r := &RedisDB{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling Close with nil Client")
		}
	}()
	r.Close()
}

func TestPostgresDB_HealthCheck_NilContext(t *testing.T) {
	db := &PostgresDB{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Pool")
		}
	}()
	_ = db.HealthCheck(nil)
}

func TestRedisDB_HealthCheck_NilContext(t *testing.T) {
	r := &RedisDB{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Client")
		}
	}()
	_ = r.HealthCheck(nil)
}

func TestPostgresDB_HealthCheck_TimeoutContext(t *testing.T) {
	db := &PostgresDB{}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Pool and expired context")
		}
	}()
	_ = db.HealthCheck(ctx)
}

func TestRedisDB_HealthCheck_TimeoutContext(t *testing.T) {
	r := &RedisDB{}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling HealthCheck with nil Client and expired context")
		}
	}()
	_ = r.HealthCheck(ctx)
}

func TestPostgresDB_StructFields(t *testing.T) {
	db := &PostgresDB{}
	if db.Pool != nil {
		t.Error("expected nil Pool")
	}
	db2 := &PostgresDB{Pool: nil}
	if db2.Pool != nil {
		t.Error("expected nil Pool after explicit nil assignment")
	}
}

func TestRedisDB_StructFields(t *testing.T) {
	r := &RedisDB{}
	if r.Client != nil {
		t.Error("expected nil Client")
	}
	r2 := &RedisDB{Client: nil}
	if r2.Client != nil {
		t.Error("expected nil Client after explicit nil assignment")
	}
}
