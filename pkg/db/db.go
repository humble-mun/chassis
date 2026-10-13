package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Database type constants
const (
	TypeMySQL    = "mysql"
	TypePostgres = "postgres"
	TypeSQLite   = "sqlite"

	flagDBType        = "db-type"
	flagDBDSN         = "db-dsn"
	flagSkipMigration = "skip-db-migration"
)

// RegisterFlags registers database-related command line flags
func RegisterFlags(pfs *pflag.FlagSet) {
	pfs.String(flagDBType, TypeMySQL, "The database server type.")
	pfs.String(flagDBDSN, "", "The control plane database server connection string.")
	pfs.Bool(flagSkipMigration, true, "Whether to skip db schema migration.")
}

// Database errors
var (
	ErrUnsupportedDBType = fmt.Errorf("unsupported db type")
	ErrDSNNotSpecified   = fmt.Errorf("database DSN not specified")

	dialerFactories = map[string]func(string) gorm.Dialector{
		TypeMySQL:    mysql.Open,
		TypePostgres: postgres.Open,
		TypeSQLite:   sqlite.Open,
	}
	logLevelMapping = map[gormlogger.LogLevel]int{
		gormlogger.Silent: 8,
		gormlogger.Error:  1,
		gormlogger.Warn:   1,
		gormlogger.Info:   2,
	}
	migrationList = make([]interface{}, 0)
)

// RegisterMigration registers a DAO model for database migration
func RegisterMigration(dao interface{}) {
	migrationList = append(migrationList, dao)
}

// NewDB creates a new database connection and runs migrations if configured
func NewDB(rootLogger logr.Logger) (c *gorm.DB, err error) {
	dbType := viper.GetString(flagDBType)
	dsn := viper.GetString(flagDBDSN)
	skipMigration := viper.GetBool(flagSkipMigration)

	factory, supported := dialerFactories[dbType]
	if !supported {
		err = ErrUnsupportedDBType
		return
	}
	if dsn == "" {
		err = ErrDSNNotSpecified
		return
	}

	glogger := rootLogger.WithName("gorm")

	if c, err = gorm.Open(factory(dsn), &gorm.Config{
		TranslateError: true,
		Logger:         &logger{logger: glogger},
	}); err != nil {
		glogger.Error(err, "new database failed", "dbType", dbType, "dsn", dsn)
		return
	}

	glogger.V(4).Info("db connected", "dbType", dbType, "dsn", dsn,
		"skipMigration", skipMigration)

	if len(migrationList) == 0 || skipMigration {
		return
	}
	if err = c.AutoMigrate(migrationList...); err != nil {
		glogger.Error(err, "auto migrate failed", "dbType", dbType, "dsn", dsn)
		return
	}
	return
}

type logger struct {
	logger logr.Logger
}

func (l logger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	v, defined := logLevelMapping[level]
	if !defined {
		v = 3
	}
	return &logger{logger: l.logger.V(v)}
}

func (l logger) Info(_ context.Context, format string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l logger) Warn(_ context.Context, format string, args ...interface{}) {
	l.logger.V(1).Info(fmt.Sprintf(format, args...))
}

func (l logger) Error(_ context.Context, format string, args ...interface{}) {
	l.logger.V(1).Info(fmt.Sprintf(format, args...))
}

func (l logger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	var trace logr.Logger
	if trace = l.logger.V(4); !trace.Enabled() && err == nil {
		return
	}
	elapsed := time.Since(begin)
	if elapsed > 200*time.Millisecond {
		trace = l.logger
	}
	rawSQL, rows := fc()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			l.logger.Error(err, "trace", "elapsed", elapsed, "sql", rawSQL, "rows", rows)
			return
		}
	}
	trace.Info("trace", "elapsed", elapsed, "sql", rawSQL, "rows", rows)
}
