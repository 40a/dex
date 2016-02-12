package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-gorp/gorp"

	"github.com/coreos/dex/connector"
	"github.com/coreos/dex/repo"
)

const (
	connectorConfigTableName = "connector_config"
)

func init() {
	register(table{
		name:    connectorConfigTableName,
		model:   connectorConfigModel{},
		autoinc: false,
		pkey:    []string{"id"},
	})
}

func newConnectorConfigModel(cfg connector.ConnectorConfig) (*connectorConfigModel, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	m := &connectorConfigModel{
		ID:     cfg.ConnectorID(),
		Type:   cfg.ConnectorType(),
		Config: string(b),
	}

	return m, nil
}

type connectorConfigModel struct {
	ID     string `db:"id"`
	Type   string `db:"type"`
	Config string `db:"config"`
}

func (m *connectorConfigModel) ConnectorConfig() (connector.ConnectorConfig, error) {
	cfg, err := connector.NewConnectorConfigFromType(m.Type)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal([]byte(m.Config), cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func NewConnectorConfigRepo(dbm *gorp.DbMap) *ConnectorConfigRepo {
	return &ConnectorConfigRepo{dbMap: dbm}
}

type ConnectorConfigRepo struct {
	dbMap *gorp.DbMap
}

func (r *ConnectorConfigRepo) All() ([]connector.ConnectorConfig, error) {
	qt := r.dbMap.Dialect.QuotedTableForQuery("", connectorConfigTableName)
	q := fmt.Sprintf("SELECT * FROM %s", qt)
	objs, err := r.dbMap.Select(&connectorConfigModel{}, q)
	if err != nil {
		return nil, err
	}

	cfgs := make([]connector.ConnectorConfig, len(objs))
	for i, obj := range objs {
		m, ok := obj.(*connectorConfigModel)
		if !ok {
			return nil, errors.New("unable to cast connector to connectorConfigModel")
		}

		cfg, err := m.ConnectorConfig()
		if err != nil {
			return nil, err
		}
		cfgs[i] = cfg
	}

	return cfgs, nil
}

func (r *ConnectorConfigRepo) GetConnectorByID(tx repo.Transaction, id string) (connector.ConnectorConfig, error) {
	qt := r.dbMap.Dialect.QuotedTableForQuery("", connectorConfigTableName)
	q := fmt.Sprintf("SELECT * FROM %s WHERE id = $1", qt)
	var c connectorConfigModel
	if err := executor(r.dbMap, tx).SelectOne(&c, q, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, connector.ErrorNotFound
		}
		return nil, err
	}
	return c.ConnectorConfig()
}

func (r *ConnectorConfigRepo) Set(cfgs []connector.ConnectorConfig) error {
	insert := make([]interface{}, len(cfgs))
	for i, cfg := range cfgs {
		m, err := newConnectorConfigModel(cfg)
		if err != nil {
			return err
		}

		insert[i] = m
	}

	tx, err := r.dbMap.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qt := r.dbMap.Dialect.QuotedTableForQuery("", connectorConfigTableName)
	q := fmt.Sprintf("DELETE FROM %s", qt)
	if _, err = tx.Exec(q); err != nil {
		return err
	}

	if err = tx.Insert(insert...); err != nil {
		return fmt.Errorf("DB insert failed %#v: %v", insert, err)
	}

	return tx.Commit()
}
