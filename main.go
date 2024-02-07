package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type TableSchema struct {
	ColumnName             string
	UdtName                string
	CharacterMaximumLength *int
	IsNullable             string
	ColumnDefault          *string
}

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		panic("no db url")
	}

	url := args[1]

	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}

	var tables []struct {
		TableName  string
		PrimaryKey string
	}

	err = db.Select("table_name, pg_get_constraintdef(oid) primary_key").Table(`information_schema."tables" t`).
		Joins("INNER JOIN pg_constraint pc ON pc.conrelid::regclass::text = t.table_name", "").
		Where(`table_type = 'BASE TABLE'`).
		Where(`table_schema not in ('pg_catalog', 'information_schema')`).
		Where(`pc.contype = 'p'`).
		Find(&tables).Error

	if err != nil {
		panic(err)
	}

	// clean primary key and generate relation key
	rgx := regexp.MustCompile("PRIMARY KEY \\((.*)\\)")
	relationship := make(map[string]string)
	for i, table := range tables {
		pk := rgx.FindStringSubmatch(table.PrimaryKey)[1]
		tables[i].PrimaryKey = pk

		relationship[fmt.Sprintf("%s_%s", table.TableName, pk)] = fmt.Sprintf("%s.%s", table.TableName, pk)
	}

	res := ""
	for _, table := range tables {
		var schema []TableSchema
		err = db.Select("*").Table(`information_schema."columns"`).Where("table_name = ?", table.TableName).Find(&schema).Error
		if err != nil {
			panic(err)
		}

		//get primary key

		fields := make([]string, 0)
		for _, column := range schema {
			field := fmt.Sprintf("\t%s %s", column.ColumnName, column.UdtName)
			if column.CharacterMaximumLength != nil {
				field = fmt.Sprintf("%s(%d)", field, *column.CharacterMaximumLength)
			}

			constraints := make([]string, 0)
			if column.ColumnName == table.PrimaryKey {
				constraints = append(constraints, "pk")
			}

			if ref, ok := relationship[column.ColumnName]; ok {
				constraints = append(constraints, fmt.Sprintf("ref: > %s", ref))
			}

			if column.IsNullable == "NO" {
				constraints = append(constraints, "NOT NULL")
			}

			if column.ColumnDefault != nil {
				constraints = append(constraints, fmt.Sprintf("default: `%s`", *column.ColumnDefault))
			}

			if len(constraints) > 0 {
				field = fmt.Sprintf("%s [%s]", field, strings.Join(constraints, ", "))
			}

			fields = append(fields, field)
		}

		dbDiagram := fmt.Sprintf("Table %s {\n%s\n}", table.TableName, strings.Join(fields, "\n"))
		res = fmt.Sprintf("%s\n\n%s", res, dbDiagram)
	}

	fmt.Println(res)
}
