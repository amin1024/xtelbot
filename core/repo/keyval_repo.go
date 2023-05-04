package repo

import (
	"context"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

func GetKeyVal(key string) ([]*models.Keyval, error) {
	return models.Keyvals(models.KeyvalWhere.Key.EQ(key)).All(context.Background(), db)
}

func SetKeyVal(key, value string) error {
	r := models.Keyval{
		Key:   key,
		Value: value,
	}
	return r.Insert(context.Background(), db, boil.Infer())
}
