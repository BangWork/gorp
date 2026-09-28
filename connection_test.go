package gorp

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

type connectionProbe struct {
	options   driver.TxOptions
	rollbacks int
	commits   int
	closed    int
}

func (p *connectionProbe) Connect(context.Context) (driver.Conn, error) { return p, nil }
func (p *connectionProbe) Driver() driver.Driver                        { return p }
func (p *connectionProbe) Open(string) (driver.Conn, error)             { return p, nil }
func (p *connectionProbe) Close() error                                 { p.closed++; return nil }
func (p *connectionProbe) Prepare(string) (driver.Stmt, error)          { return nil, errors.New("unused") }
func (p *connectionProbe) Begin() (driver.Tx, error)                    { return p, nil }
func (p *connectionProbe) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	p.options = opts
	return p, nil
}
func (p *connectionProbe) Commit() error   { p.commits++; return nil }
func (p *connectionProbe) Rollback() error { p.rollbacks++; return nil }

func TestConnectionTransactionBoundary(t *testing.T) {
	probe := &connectionProbe{}
	m := &DbMap{Db: sql.OpenDB(probe)}
	defer m.Db.Close()
	tx, err := m.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if tx.DBMap() != m || tx.RawTx() != tx.tx || probe.options != (driver.TxOptions{}) {
		t.Fatalf("wrong pool, transaction or default options: %+v", probe.options)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	explicit := &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true}
	tx, err = m.BeginTx(context.Background(), explicit)
	if err != nil {
		t.Fatal(err)
	}
	if probe.options.Isolation != driver.IsolationLevel(sql.LevelReadCommitted) || !probe.options.ReadOnly {
		t.Fatalf("explicit options lost: %+v", probe.options)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.BeginTx(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled BeginTx: %v", err)
	}
}

func TestPostBeginFailureReleasesConnection(t *testing.T) {
	probe := &connectionProbe{}
	m := &DbMap{Db: sql.OpenDB(probe)}
	defer m.Db.Close()
	m.Db.SetMaxOpenConns(1)
	m.SetDbBrand("test")
	want := errors.New("hook failed")
	m.SetPostBeginHooks(func(tx *Transaction) error {
		if tx.DBMap() != m {
			t.Fatal("hook received a transaction from another map")
		}
		return want
	})
	if tx, err := m.Begin(); tx != nil || !errors.Is(err, want) {
		t.Fatalf("Begin: tx=%v err=%v", tx, err)
	}
	if probe.rollbacks != 1 || m.Db.Stats().InUse != 0 {
		t.Fatalf("failed hook leaked transaction: rollbacks=%d stats=%+v", probe.rollbacks, m.Db.Stats())
	}
}
