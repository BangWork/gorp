module github.com/go-gorp/gorp/tests

go 1.18

require (
	github.com/go-sql-driver/mysql v1.6.0
	github.com/lib/pq v1.10.7
	github.com/mattn/go-sqlite3 v1.14.15
	github.com/ziutek/mymysql v1.5.4
	gopkg.in/gorp.v1 v1.0.0
)

replace gopkg.in/gorp.v1 => ../
