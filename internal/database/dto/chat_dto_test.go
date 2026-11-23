package dto

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestMessageDtoMapsIVColumnOnlyToIVBytes(t *testing.T) {
	modelSchema, err := schema.Parse(MessageDto{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse GORM schema: %v", err)
	}

	ivField, ok := modelSchema.FieldsByDBName["iv"]
	if !ok {
		t.Fatal("column \"iv\" must be mapped to a schema field")
	}
	if ivField.Name != "IVBytes" {
		t.Fatalf("column \"iv\" must map to IVBytes, got field %q — a duplicate mapping would make GORM scan raw BYTEA bytes into the JSON string field and corrupt the API response", ivField.Name)
	}

	if jsonField, ok := modelSchema.FieldsByDBName["IV"]; ok {
		t.Fatalf("field IV must not be DB-mapped, found column %q", jsonField.DBName)
	}
}
