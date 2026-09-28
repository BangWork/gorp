package gorp

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestPostgresDialectInsertSQL(t *testing.T) {
	type item struct {
		ID    int64  `db:"id"`
		Value string `db:"select"`
	}
	for _, test := range []struct {
		name    string
		dialect PostgresDialect
		query   string
		sqlType string
		auto    string
	}{
		{
			name: "native", dialect: PostgresDialect{},
			query:   `insert into app."order" ("id","select") values (default,$1) returning "id";`,
			sqlType: "bigserial",
		},
		{
			name: "compatible", dialect: PostgresDialect{CompatibleSQL: true},
			query:   "insert into app.`order` (`id`,`select`) values (default,?);",
			sqlType: "bigint", auto: "auto_increment",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := &DbMap{Dialect: test.dialect}
			table := m.AddTableWithNameAndSchema(item{}, "app", "order").SetKeys(true, "ID")
			bound, err := table.bindInsert(reflect.ValueOf(item{Value: "value"}))
			if err != nil {
				t.Fatal(err)
			}
			if bound.query != test.query || !reflect.DeepEqual(bound.args, []interface{}{"value"}) {
				t.Fatalf("insert: query=%q args=%v", bound.query, bound.args)
			}
			if got := test.dialect.ToSqlType(reflect.TypeOf(int64(0)), 0, true); got != test.sqlType {
				t.Fatalf("column type: %q", got)
			}
			if got := test.dialect.AutoIncrStr(); got != test.auto {
				t.Fatalf("auto increment: %q", got)
			}
			// Adding the integer inserter would take precedence over RETURNING
			// in gorp's dispatch, even when the dialect is in native mode.
			if _, ok := interface{}(test.dialect).(IntegerAutoIncrInserter); ok {
				t.Fatal("PostgresDialect must retain targeted insert dispatch")
			}
		})
	}
}

type postgresInsertExecutor struct {
	SqlExecutor
	result sql.Result
	err    error
	calls  int
}

func (e *postgresInsertExecutor) Exec(string, ...interface{}) (sql.Result, error) {
	e.calls++
	return e.result, e.err
}

type postgresInsertResult struct {
	id  int64
	err error
}

func (r postgresInsertResult) LastInsertId() (int64, error) { return r.id, r.err }
func (r postgresInsertResult) RowsAffected() (int64, error) { return 1, nil }

func TestPostgresCompatibleGeneratedID(t *testing.T) {
	type namedID int64
	type item struct{ ID namedID }
	dialect := PostgresDialect{CompatibleSQL: true}
	m := &DbMap{Dialect: dialect}
	m.AddTable(item{}).SetKeys(true, "ID")
	row := &item{}
	exec := &postgresInsertExecutor{result: postgresInsertResult{id: 42}}
	if err := insert(m, exec, row); err != nil || row.ID != 42 || exec.calls != 1 {
		t.Fatalf("generated ID: row=%+v calls=%d err=%v", row, exec.calls, err)
	}
	var unsigned uint64
	if err := dialect.InsertAutoIncrToTarget(exec, "insert", &unsigned); err != nil || unsigned != 42 {
		t.Fatalf("unsigned ID: %d %v", unsigned, err)
	}
	for _, stage := range []string{"exec", "result"} {
		t.Run(stage, func(t *testing.T) {
			want := errors.New("insert failed")
			exec := &postgresInsertExecutor{result: postgresInsertResult{err: want}}
			if stage == "exec" {
				exec.err = want
			}
			row := &item{ID: 7}
			if err := insert(m, exec, row); !errors.Is(err, want) || row.ID != 7 || exec.calls != 1 {
				t.Fatalf("insert error: row=%+v calls=%d err=%v", row, exec.calls, err)
			}
		})
	}
}

func TestPostgresCompatibleGeneratedIDRejectsInvalidTargets(t *testing.T) {
	dialect := PostgresDialect{CompatibleSQL: true}
	var nilInt *int64
	for _, target := range []interface{}{nil, nilInt, 1, new(string)} {
		exec := &postgresInsertExecutor{result: postgresInsertResult{id: 42}}
		if err := dialect.InsertAutoIncrToTarget(exec, "insert", target); err == nil || exec.calls != 0 {
			t.Fatalf("invalid target %T: calls=%d err=%v", target, exec.calls, err)
		}
	}
	for _, test := range []struct {
		id     int64
		target interface{}
	}{
		{id: 128, target: new(int8)},
		{id: 256, target: new(uint8)},
		{id: -1, target: new(uint64)},
	} {
		exec := &postgresInsertExecutor{result: postgresInsertResult{id: test.id}}
		if err := dialect.InsertAutoIncrToTarget(exec, "insert", test.target); err == nil {
			t.Fatalf("ID %d did not overflow %T", test.id, test.target)
		}
		if !reflect.ValueOf(test.target).Elem().IsZero() {
			t.Fatalf("overflow changed target %T", test.target)
		}
	}
}
