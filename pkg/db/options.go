package db

import (
	"flag"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"k8s.io/apimachinery/pkg/util/errors"
)

type Options struct {
	dbPort     int
	dbUsername string
	dbPassword string
	dbHost     string
	dbName     string
	dbLogLevel int
}

func (o *Options) Bind(fs *flag.FlagSet) {
	fs.IntVar(&o.dbPort, "db-port", 5432, "Database port number")
	fs.StringVar(&o.dbUsername, "db-username", "", "Database username")
	fs.StringVar(&o.dbPassword, "db-password", "", "Database password")
	fs.StringVar(&o.dbHost, "db-host", "", "Database host")
	fs.StringVar(&o.dbName, "db-name", "", "Database name")
	fs.IntVar(&o.dbLogLevel, "db-log-level", 1, "Database log level")
}

func (o *Options) Validate() error {
	var errs []error
	if o.dbUsername == "" {
		errs = append(errs, fmt.Errorf("--db-username is not specified"))
	}
	if o.dbPassword == "" {
		errs = append(errs, fmt.Errorf("--db-password is not specified"))
	}
	if o.dbHost == "" {
		errs = append(errs, fmt.Errorf("--db-host is not specified"))
	}
	if o.dbName == "" {
		errs = append(errs, fmt.Errorf("--db-name is not specified"))
	}
	return errors.NewAggregate(errs)
}

func (o *Options) connect() (*gorm.DB, error) {
	url := fmt.Sprintf("host=%s port=%v user=%s dbname=%s password=%s sslmode=disable",
		o.dbHost,
		o.dbPort,
		o.dbUsername,
		o.dbName,
		o.dbPassword)

	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.LogLevel(o.dbLogLevel)),
	})
	return db.Session(&gorm.Session{
		FullSaveAssociations: true,
		QueryFields:          true,
	}), err
}

func (o *Options) NewCalculationResultsStore() (CalculationResultsStore, error) {
	db, err := o.connect()
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize database: %w", err)
	}
	if err = db.AutoMigrate(&CalculationResults{}); err != nil {
		return nil, err
	}
	return &calculationResultsStore{db: db}, nil
}
