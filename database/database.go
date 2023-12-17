package database

import (
	"database/sql"
	"github.com/upper/db/v4"
	"github.com/upper/db/v4/adapter/postgresql"
	commonmodel "notification-service/common/common-model"
)

type Database struct {
	conn   db.Session
	config db.ConnectionURL
}

type DatabaseInterface interface {
	UpdateClientServiceId(client string, serviceId string) error
	UpdateClientServiceIdToNull(client string) error
}

func GetNewDatabaseConnection(connUrl db.ConnectionURL) *Database {
	session, err := postgresql.Open(connUrl)
	if err != nil {
		panic(err)
	}
	return &Database{conn: session, config: connUrl}
}

func (d Database) UpdateClientServiceId(client string, serviceId string) error {
	q := d.conn.SQL().Update("notifier_instances").
		Set("notifier_instance_id", serviceId).
		Where("user_id = ?", client)
	_, err := q.Exec()
	if err != nil {
		return commonmodel.ErrDbUnexpected
	}
	return nil
}

func (d Database) UpdateClientServiceIdToNull(client string) error {
	q := d.conn.SQL().Update("notifier_instances").
		Set("notifier_instance_id", sql.NullString{}).
		Where("user_id = ?", client)
	_, err := q.Exec()
	if err != nil {
		return commonmodel.ErrDbUnexpected
	}
	return nil
}
