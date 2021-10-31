package parser

import (
	"bitreader"
	"log"
	"reader"
)

type DataManager struct {
	m  reader.M
	br *bitreader.BitReader
}

func NewDataManager(br *bitreader.BitReader) *DataManager {
	return &DataManager{br: br, m: reader.M{}}
}

func (dm *DataManager) _decode(input, output reader.M) error {
	for k, v := range input {
		_v, ok := v.(int)
		if !ok {
			continue
		}
		ret, err := dm.br.Read(uint(_v))
		if err != nil {
			log.Println("read", k, v, "err")
			return err
		}
		output[k] = ret
	}
	return nil
}

func (dm *DataManager) decode(input reader.M) error {
	return dm._decode(input, dm.m)
}

func (dm *DataManager) decodeChild(m reader.M) error {
	return dm._decode(m, m)
}

func (dm *DataManager) set(key string, val interface{}) {
	dm.m[key] = val
}

func (dm *DataManager) get(key string) uint64 {
	return dm.m[key].(uint64)
}

func (dm *DataManager) read(key string, len uint) uint64 {
	val, _ := dm.br.Read(len)
	dm.set(key, val)
	return val
}

func (dm *DataManager) data() reader.M {
	return dm.m
}
