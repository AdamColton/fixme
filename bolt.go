package fixme

import (
	"encoding/gob"
	"github.com/adamcolton/gothic/bufpool"
	"github.com/boltdb/bolt"
)

var db *bolt.DB

var (
	projectsBucket = []byte("pb")
	settingsBucket = []byte("st")
)

func boltInit() {
	var err error
	db, err = bolt.Open("projects.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(projectsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(settingsBucket); err != nil {
			return err
		}
		return nil
	})
}

func (p *Project) Save() {
	if db == nil {
		boltInit()
	}

	buf := bufpool.Get()
	pr := p.ProjectRecord()
	pr.ID = nil
	gob.NewEncoder(buf).Encode(pr)
	data := buf.Bytes()
	bufpool.Put(buf)

	db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(projectsBucket)
		bkt.Put(p.id, data)
		return nil
	})
}

func (p *Project) Delete() {
	if db == nil {
		boltInit()
	}

	db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(projectsBucket)
		bkt.Delete(p.id)
		return nil
	})
}

func Load(id []byte) *Project {
	if db == nil {
		boltInit()
	}

	var data []byte
	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(projectsBucket)
		if id == nil {
			c := bkt.Cursor()
			id, data = c.First()
		} else {
			data = bkt.Get(id)
		}
		return nil
	})

	if id == nil {
		return NewProject()
	}

	buf := bufpool.Get()
	buf.Write(data)
	var pr ProjectRecord
	gob.NewDecoder(buf).Decode(&pr)
	bufpool.Put(buf)

	return pr.Project(id)
}

func List() []ProjectRecord {
	if db == nil {
		boltInit()
	}

	projects := make([]ProjectRecord, 0)
	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(projectsBucket)
		c := bkt.Cursor()
		for id, data := c.First(); id != nil; id, data = c.Next() {
			buf := bufpool.Get()
			buf.Write(data)
			var pr ProjectRecord
			gob.NewDecoder(buf).Decode(&pr)
			bufpool.Put(buf)

			pr.Pkgs = nil
			pr.ID = id
			projects = append(projects, pr)
		}
		return nil
	})

	return projects
}
