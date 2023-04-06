package repo

import (
	"context"
	"github.com/amin1024/xtelbot/core/repo/models"
)

var defaultPackage *models.Package

func SetupPackage() {
	dp, err := models.Packages(models.PackageWhere.Name.EQ("basic_test")).One(context.Background(), db)
	if err != nil {
		panic(err)
	}
	defaultPackage = dp
}

func GetPackage(name string) (*models.Package, error) {
	if name == "" {
		return defaultPackage, nil
	}
	p, err := models.Packages(models.PackageWhere.Name.EQ(name)).One(context.Background(), db)
	if err != nil {
		return nil, err
	}
	return p, nil
}
