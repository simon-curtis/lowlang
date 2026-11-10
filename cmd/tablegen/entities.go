package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

const maxEntityName = 32

type entity struct {
	Characters string `json:"characters"`
}

func generateEntities(source, output string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	var table map[string]entity
	if err := json.Unmarshal(data, &table); err != nil {
		return err
	}
	names := make([]string, 0, len(table))
	for name := range table {
		names = append(names, name)
	}
	slices.Sort(names)
	var index, nameBytes, valueBytes []byte
	for _, name := range names {
		bare := strings.TrimPrefix(name, "&")
		value := table[name].Characters
		if len(bare) > maxEntityName || len(value) > 0xFF {
			return fmt.Errorf("entity %s exceeds the table's limits", name)
		}
		index = appendU16(index, len(nameBytes))
		index = append(index, byte(len(bare)))
		index = appendU16(index, len(valueBytes))
		index = append(index, byte(len(value)))
		nameBytes = append(nameBytes, bare...)
		valueBytes = append(valueBytes, value...)
	}
	if len(nameBytes) > 0xFFFF || len(valueBytes) > 0xFFFF {
		return fmt.Errorf("entity names or values overflow 16-bit offsets")
	}
	return writeSections(output, []section{
		{"entity_index.bin", index},
		{"entity_names.bin", nameBytes},
		{"entity_values.bin", valueBytes},
	})
}
