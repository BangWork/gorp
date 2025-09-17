package gorp

import (
	"reflect"
	"strings"
	"testing"
)

func TestBindUpdateSkipsPrimaryKey(t *testing.T) {
	type user struct {
		Id   int64
		Name string
	}

	dbmap := &DbMap{Dialect: MySQLDialect{Engine: "InnoDB", Encoding: "UTF8"}}
	table := dbmap.AddTableWithName(user{}, "users").SetKeys(false, "Id")

	u := user{Id: 41, Name: "Alice"}
	bi, err := table.bindUpdate(reflect.ValueOf(&u).Elem())
	if err != nil {
		t.Fatalf("bindUpdate error: %v", err)
	}

	if strings.Contains(strings.ToLower(bi.query), "set `id`") {
		t.Fatalf("expected primary key to be excluded from SET clause, got query: %s", bi.query)
	}
	if !strings.Contains(strings.ToLower(bi.query), "where `id`") {
		t.Fatalf("expected WHERE clause to reference primary key, got query: %s", bi.query)
	}

	if len(bi.args) != 2 {
		t.Fatalf("expected 2 bind args (columns + key), got %d", len(bi.args))
	}
	if bi.args[0] != u.Name {
		t.Fatalf("expected first arg to be Name value, got %#v", bi.args[0])
	}
	if bi.args[1] != u.Id {
		t.Fatalf("expected second arg to be Id value, got %#v", bi.args[1])
	}
}

func TestBindUpdateNoUpdatableColumns(t *testing.T) {
	type onlyID struct {
		Id int64
	}

	dbmap := &DbMap{Dialect: MySQLDialect{Engine: "InnoDB", Encoding: "UTF8"}}
	table := dbmap.AddTableWithName(onlyID{}, "only_ids").SetKeys(false, "Id")

	rec := onlyID{Id: 7}
	if _, err := table.bindUpdate(reflect.ValueOf(&rec).Elem()); err == nil {
		t.Fatalf("expected error when no columns can be updated")
	}
}
