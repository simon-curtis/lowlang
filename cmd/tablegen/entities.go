package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

const (
	maxEntityName           = 32
	entityNameOffset        = 0
	entityNameLengthOffset  = entityNameOffset + u16Width
	entityValueOffset       = entityNameLengthOffset + 1
	entityValueLengthOffset = entityValueOffset + u16Width
	entitySize              = entityValueLengthOffset + 1
)

var entityTables = []string{"entity_index.bin", "entity_names.bin", "entity_values.bin"}

const entityLayoutFile = "entity_layout.low"

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
		index = append(index, encodeEntity(len(nameBytes), len(bare), len(valueBytes), len(value))...)
		nameBytes = append(nameBytes, bare...)
		valueBytes = append(valueBytes, value...)
	}
	if len(nameBytes) > 0xFFFF || len(valueBytes) > 0xFFFF {
		return fmt.Errorf("entity names or values overflow 16-bit offsets")
	}
	sections := namedSections(entityTables, index, nameBytes, valueBytes)
	sections = append(sections, section{entityLayoutFile, []byte(entityLayoutSource())})
	return writeSections(output, sections)
}

func encodeEntity(nameAt, nameLength, valueAt, valueLength int) []byte {
	out := make([]byte, entitySize)
	binary.LittleEndian.PutUint16(out[entityNameOffset:], uint16(nameAt))
	out[entityNameLengthOffset] = byte(nameLength)
	binary.LittleEndian.PutUint16(out[entityValueOffset:], uint16(valueAt))
	out[entityValueLengthOffset] = byte(valueLength)
	return out
}

func entityLayoutSource() string {
	constants := []layoutConstant{
		lowConstant("max_entity_name", maxEntityName, lowInt),
		lowConstant("entity_size", entitySize, lowInt),
		lowConstant("entity_name_offset", entityNameOffset, lowInt),
		lowConstant("entity_name_length_offset", entityNameLengthOffset, lowInt),
		lowConstant("entity_value_offset", entityValueOffset, lowInt),
		lowConstant("entity_value_length_offset", entityValueLengthOffset, lowInt),
	}
	return layoutSource(constants, entityTables)
}
