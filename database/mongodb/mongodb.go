package mongodb

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/leochen2038/play"
	"github.com/leochen2038/play/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var dbconnects sync.Map

func getConnect(ctx context.Context, router string) (*mongo.Client, error) {
	var err error
	var mongoURI string
	var dest string
	var dbconnect *mongo.Client

	if dest, err = config.String(router); err != nil {
		return nil, fmt.Errorf("can not find mongodb config:" + router)
	}
	if connect, _ := dbconnects.Load(dest); connect == nil {
		scheme := "mongodb"
		username, password, host, _ := play.DecodeHost(scheme, dest)
		if username == "" {
			mongoURI = scheme + "://" + host
		} else {
			mongoURI = scheme + "://" + username + ":" + password + "@" + host
		}

		if dbconnect, err = mongo.NewClient(options.Client().ApplyURI(mongoURI).SetMaxPoolSize(1024)); err != nil {
			return nil, err
		}

		if err = dbconnect.Connect(ctx); err != nil {
			return nil, err
		}

		if connect, ok := dbconnects.LoadOrStore(dest, dbconnect); ok {
			return connect.(*mongo.Client), nil
		}

		return dbconnect, nil
	} else {
		return connect.(*mongo.Client), nil
	}
}

func getCollection(query *play.Query) (collection *mongo.Collection, err error) {
	var client *mongo.Client
	if client, err = getConnect(context.Background(), query.Router); err != nil {
		return nil, err
	}
	collection = client.Database(query.DBName).Collection(query.Table)
	return
}

func GetList(dest interface{}, query *play.Query) (err error) {
	var collection *mongo.Collection
	if collection, err = getCollection(query); err != nil {
		return
	}

	var cursor *mongo.Cursor
	filter := fetch(query)
	options := findOptions(query)

	if cursor, err = collection.Find(query.Context, filter, options); err != nil {
		return
	}

	defer cursor.Close(context.Background())
	err = cursor.All(query.Context, dest)

	return
}

func GetOne(dest interface{}, query *play.Query) (err error) {
	var collection *mongo.Collection
	if collection, err = getCollection(query); err != nil {
		return
	}

	filter := fetch(query)
	options := findOneOptions(query)

	err = collection.FindOne(query.Context, filter, options).Decode(dest)
	if err == mongo.ErrNoDocuments {
		return play.ErrQueryEmptyResult
	}
	return
}

func UpdateAndGetOne(dest interface{}, query *play.Query) (err error) {
	var collection *mongo.Collection
	if collection, err = getCollection(query); err != nil {
		return
	}

	fmtime := make([]interface{}, 0, 1)
	fmtime = append(fmtime, time.Now().Unix())
	query.Sets["Fmtime"] = fmtime

	filter := fetch(query)
	update := modifier(query)

	err = collection.FindOneAndUpdate(query.Context, filter, update).Decode(dest)
	if err == mongo.ErrNoDocuments {
		return play.ErrQueryEmptyResult
	}
	return
}

func Save(meta interface{}, upsetId *primitive.ObjectID, query *play.Query) (err error) {
	var collection *mongo.Collection
	if collection, err = getCollection(query); err != nil {
		return
	}

	if upsetId == nil {
		_, err = collection.InsertOne(query.Context, meta)
	} else {
		filter := bson.M{"_id": upsetId}
		_, err = collection.ReplaceOne(query.Context, filter, meta)
	}

	return
}

func Delete(query *play.Query) (modcount int64, err error) {
	var result *mongo.DeleteResult
	var collection *mongo.Collection

	fmtime := make([]interface{}, 0, 1)
	fmtime = append(fmtime, time.Now().Unix())
	query.Sets["Fmtime"] = fmtime

	if collection, err = getCollection(query); err != nil {
		return
	}

	filter := fetch(query)

	if result, err = collection.DeleteMany(query.Context, filter); err != nil {
		return
	}

	return result.DeletedCount, nil
}

func Update(query *play.Query) (modcount int64, err error) {
	var result *mongo.UpdateResult
	var collection *mongo.Collection

	fmtime := make([]interface{}, 0, 1)
	fmtime = append(fmtime, time.Now().Unix())
	query.Sets["Fmtime"] = fmtime

	if collection, err = getCollection(query); err != nil {
		return
	}

	filter := fetch(query)
	update := modifier(query)
	if result, err = collection.UpdateMany(query.Context, filter, update); err != nil {
		return
	}

	return result.ModifiedCount, nil
}

func SaveList(metaList interface{}, query *play.Query) (err error) {
	var collection *mongo.Collection
	if collection, err = getCollection(query); err != nil {
		return
	}
	var writes []mongo.WriteModel

	for _, meta := range metaList.([]interface{}) {
		writes = append(writes, mongo.NewInsertOneModel().SetDocument(meta))
	}
	_, err = collection.BulkWrite(query.Context, writes)
	return
}

func Count(query *play.Query) (count int64, err error) {
	var collection *mongo.Collection
	if collection, err = getCollection(query); err != nil {
		return
	}

	filter := fetch(query)

	count, err = collection.CountDocuments(query.Context, filter)

	return
}

func modifier(query *play.Query) bson.M {
	var mod bson.M = bson.M{}
	var set bson.M = bson.M{}
	var inc bson.M = bson.M{}
	for key, item := range query.Sets {
		if len(item) == 2 {
			ftype := fieldType(item[0])
			switch ftype {
			case "int":
				if item[1] == "@+" {
					inc[key] = item[0]
				} else if item[1] == "@-" {
					inc[key] = -item[0].(int)
				}
			case "float":
				if item[1] == "@+" {
					inc[key] = item[0]
				} else if item[1] == "@-" {
					inc[key] = -item[0].(float32)
				}
			}
		} else {
			set[key] = item[0]
		}
	}

	if len(inc) > 0 {
		mod["$inc"] = inc
	}

	if len(set) > 0 {
		mod["$set"] = set
	}
	return mod
}

func fieldType(item interface{}) string {
	switch item.(type) {
	case int:
		return "int"
	case int8:
		return "int"
	case int16:
		return "int"
	case int32:
		return "int"
	case int64:
		return "int"
	case float32:
		return "float"
	case float64:
		return "float"
	}
	return ""
}

func findOptions(query *play.Query) *options.FindOptions {
	options := &options.FindOptions{}
	if query.Limit[1] > 0 {
		options.SetSkip(query.Limit[0])
		options.SetLimit(query.Limit[1])
	}

	if len(query.Order) > 0 {
		var sortFields bson.D
		for i := 0; i < len(query.Order); i++ {
			sort := 1
			if query.Order[i][1] == "desc" {
				sort = -1
			}
			sortFields = append(sortFields, bson.E{query.Order[i][0], sort})
		}
		options.SetSort(sortFields)
	}
	return options
}

func findOneOptions(query *play.Query) *options.FindOneOptions {
	options := options.FindOneOptions{}
	if query.Limit[1] > 0 {
		options.SetSkip(query.Limit[0])
	}

	if len(query.Order) > 0 {
		var sortFields bson.D
		for i := 0; i < len(query.Order); i++ {
			sort := 1
			if query.Order[i][1] == "desc" {
				sort = -1
			}
			sortFields = append(sortFields, bson.E{query.Order[i][0], sort})
		}
		options.SetSort(sortFields)
	}
	return &options
}

func fetch(query *play.Query) bson.M {
	var filter = bson.M{}
	for _, cond := range query.Conditions {
		var fieldCon bson.M
		if _, ok := filter[cond.Field]; !ok {
			filter[cond.Field] = bson.M{}
		}
		fieldCon = filter[cond.Field].(bson.M)
		switch cond.Con {
		case "Equal":
			if cond.Field == "_id" && reflect.TypeOf(cond.Val).String() == "string" {
				fieldCon["$eq"], _ = primitive.ObjectIDFromHex(cond.Val.(string))
			} else {
				fieldCon["$eq"] = cond.Val
			}
		case "NotEqual":
			if cond.Field == "_id" && reflect.TypeOf(cond.Val).String() == "string" {
				fieldCon["$ne"], _ = primitive.ObjectIDFromHex(cond.Val.(string))
			} else {
				fieldCon["$ne"] = cond.Val
			}
		case "NotIn":
			if cond.Field == "_id" {
				if reflect.TypeOf(cond.Val).String() == "[]interface {}" {
					list := make([]primitive.ObjectID, 0, 1)
					for _, v := range cond.Val.([]interface{}) {
						switch v.(type) {
						case primitive.ObjectID:
							list = append(list, v.(primitive.ObjectID))
						case string:
							obj, _ := primitive.ObjectIDFromHex(v.(string))
							list = append(list, obj)
						}
					}
					fieldCon["$nin"] = list
				} else {
					fieldCon["$nin"] = cond.Val
				}
			} else {
				fieldCon["$nin"] = cond.Val
			}
		case "Less":
			fieldCon["$lt"] = cond.Val
		case "Greater":
			fieldCon["$gt"] = cond.Val
		case "In":
			if cond.Field == "_id" {
				if reflect.TypeOf(cond.Val).String() == "[]interface {}" {
					list := make([]primitive.ObjectID, 0, 1)
					for _, v := range cond.Val.([]interface{}) {
						switch v.(type) {
						case primitive.ObjectID:
							list = append(list, v.(primitive.ObjectID))
						case string:
							obj, _ := primitive.ObjectIDFromHex(v.(string))
							list = append(list, obj)
						}
					}
					fieldCon["$in"] = list
				} else {
					fieldCon["$in"] = cond.Val
				}
			} else {
				fieldCon["$in"] = cond.Val
			}
		case "Like":
			regex := (cond.Val).(string)
			strings.ReplaceAll(regex, "%", ".*")
			fieldCon["$regex"] = strings.ReplaceAll(regex, "%", ".*")
			fieldCon["$options"] = "i"
		}
	}
	return filter
}
