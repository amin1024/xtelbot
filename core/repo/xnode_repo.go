package repo

import (
	"context"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

func GetXNodes() ([]*models.Xnode, error) {
	all, err := models.Xnodes().All(context.Background(), db)
	if err != nil {
		return nil, err
	}
	return all, nil
}

func SaveOrUpdateXNode(node *models.Xnode) error {
	err := node.Upsert(
		context.Background(), db, false,
		nil,
		boil.Infer(),
		boil.Infer(),
	)
	return err
}
