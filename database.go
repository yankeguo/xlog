package xlog

import (
	"fmt"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// Database is a wrapper of mgo.Database
type Database struct {
	DB *mgo.Database

	// CollectionPrefix prefix of collections in database
	CollectionPrefix string

	// Bulk pending log entries to insert, grouped by collection name
	Bulk map[string][]LogEntry
}

// DialDatabase connect a mongo database with options
func DialDatabase(opt Options) (db *Database, err error) {
	var session *mgo.Session
	if session, err = mgo.Dial(opt.Mongo.URL); err != nil {
		return
	}
	db = &Database{
		DB:               session.DB(opt.Mongo.DB),
		CollectionPrefix: opt.Mongo.Collection,
		Bulk:             map[string][]LogEntry{},
	}
	return
}

// Close close the underlaying session
func (d *Database) Close() {
	d.DB.Session.Close()
}

// CollectionName compose collection name for LogEntry
func (d *Database) CollectionName(ts time.Time) string {
	return fmt.Sprintf(
		"%s%04d%02d%02d",
		d.CollectionPrefix,
		ts.Year(),
		ts.Month(),
		ts.Day(),
	)
}

// EnableSharding enable sharding
func (d *Database) EnableSharding(coll string) error {
	return d.DB.Run(bson.D{
		bson.DocElem{
			Name:  "shardCollection",
			Value: d.DB.Name + "." + coll,
		},
		bson.DocElem{
			Name: "key",
			Value: bson.D{bson.DocElem{
				Name:  "timestamp",
				Value: "hashed",
			}},
		},
	}, nil)
}

// EnsureIndexes ensure indexes for collection
func (d *Database) EnsureIndexes(coll string) (err error) {
	for _, field := range IndexedFields {
		if err = d.DB.C(coll).EnsureIndex(mgo.Index{
			Key:        []string{field},
			Background: true,
		}); err != nil {
			return
		}
	}
	return
}

// Insert insert a log entry, choose collection automatically
func (d *Database) Insert(le LogEntry) (err error) {
	err = d.DB.C(d.CollectionName(le.Timestamp)).Insert(le.ToBSON())
	return
}

// BulkInsertionStart start a bulk insertion with possible multiple collections invovled
func (d *Database) BulkInsertionStart() {
	// just clear
	d.BulkInsertionClear()
}

// BulkInsert record a log entry to bulk insertion
func (d *Database) BulkInsert(le LogEntry) {
	coll := d.CollectionName(le.Timestamp)
	// ensure slice exist
	if d.Bulk[coll] == nil {
		d.Bulk[coll] = []LogEntry{}
	}
	// append slice
	d.Bulk[coll] = append(d.Bulk[coll], le)
}

// BulkInsertionCommit commit the whole insertion
func (d *Database) BulkInsertionCommit() (err error) {
	// prepare slice of bson.M
	bs := []interface{}{}
	for coll, les := range d.Bulk {
		// clear and reuse slice of bson.M
		bs = bs[:0]
		// convert LogEntry to bson.M
		for _, le := range les {
			bs = append(bs, le.ToBSON())
		}
		// insert mutiple documents
		if err = d.DB.C(coll).Insert(bs...); err != nil {
			return
		}
	}
	// clear bulk
	d.BulkInsertionClear()
	return
}

// BulkInsertionClear clear a bulk insertion
func (d *Database) BulkInsertionClear() {
	// clear and resuse space of previous Bulk
	for k, v := range d.Bulk {
		d.Bulk[k] = v[:0]
	}
}