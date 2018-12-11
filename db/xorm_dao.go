package db

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-xorm/xorm"
	logger "github.com/hsyan2008/go-logger"
	"github.com/hsyan2008/hfw2/common"
	"github.com/hsyan2008/hfw2/configs"
)

var _ Dao = &XormDao{}

func NewXormDao(config configs.AllConfig, dbConfigs ...configs.DbConfig) *XormDao {
	dbConfig := config.Db

	instance := &XormDao{}

	//允许默认配置为空
	if dbConfig.Driver == "" && len(dbConfigs) == 0 {
		return instance
	}

	if len(dbConfigs) > 0 {
		dbConfig = dbConfigs[0]
	}
	instance.engine = InitDb(config, dbConfig)

	//开启缓存
	instance.cacher = GetCacher(config)

	return instance
}

type XormDao struct {
	engine  xorm.EngineInterface
	isCache bool
	cacher  *xorm.LRUCacher
}

func (d *XormDao) UpdateById(t interface{}) (err error) {
	sess := d.engine.NewSession()
	defer sess.Close()

	id := reflect.ValueOf(t).Elem().FieldByName("Id").Int()
	_, err = sess.Id(id).AllCols().Update(t)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}

	return
}

func (d *XormDao) UpdateByIds(t interface{}, params map[string]interface{},
	ids []interface{}) (err error) {

	if len(ids) == 0 {
		return errors.New("ids parameters error")
	}

	sess := d.engine.NewSession()
	defer sess.Close()
	_, err = sess.Table(t).In("id", ids).Update(params)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}

	return
}

func (d *XormDao) UpdateByWhere(t interface{}, params map[string]interface{},
	where map[string]interface{}) (err error) {
	if len(where) == 0 {
		return errors.New("where paramters error")
	}

	var (
		str  []string
		args []interface{}
	)
	for k, v := range where {
		str = append(str, fmt.Sprintf("`%s` = ?", k))
		args = append(args, v)
	}

	sess := d.engine.NewSession()
	defer sess.Close()
	_, err = sess.Table(t).Where(strings.Join(str, " AND "), args...).Update(params)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}
	return err
}

func (d *XormDao) Insert(t interface{}) (err error) {
	sess := d.engine.NewSession()
	defer sess.Close()
	_, err = sess.Insert(t)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}
	return
}

func (d *XormDao) SearchOne(t interface{}) (err error) {
	sess := d.engine.NewSession()
	defer sess.Close()
	_, err = sess.Get(t)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}
	return
}

func (d *XormDao) buildCond(cond map[string]interface{}) (sess *xorm.Session) {
	var (
		str      []string
		orderby  = "id desc"
		page     = 1
		pageSize = DefaultPageSize
		where    string
		args     []interface{}
	)
	sess = d.engine.NewSession()
FOR:
	for k, v := range cond {
		k = strings.ToLower(k)
		switch k {
		case "orderby":
			orderby = v.(string)
			continue FOR
		case "page":
			page = common.Max(v.(int), page)
			continue FOR
		case "pagesize":
			if v.(int) > 0 {
				pageSize = v.(int)
			}
			continue FOR
		case "where":
			where = v.(string)
			continue FOR
		case "sql":
			sess.SQL(v)
			continue FOR
		case "select":
			sess.Select(v.(string))
			continue FOR
		case "distinct":
			sess.Distinct(v.(string))
			continue FOR
		}
		str = append(str, fmt.Sprintf("`%s` = ?", k))
		args = append(args, v)
	}

	var strs string
	if len(str) > 0 {
		strs = strings.Join(str, " AND ")
	}

	if len(where) > 0 && len(strs) > 0 {
		where = fmt.Sprintf("(%s) AND %s", where, strs)
	} else if len(strs) > 0 {
		where = strs
	}

	return sess.Where(where, args...).OrderBy(orderby).
		Limit(pageSize, (page-1)*pageSize)
}

func (d *XormDao) Search(t interface{}, cond map[string]interface{}) (err error) {

	sess := d.buildCond(cond)
	defer sess.Close()
	err = sess.Find(t)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}

	return
}

func (d *XormDao) Rows(t interface{}, cond map[string]interface{}) (rows *xorm.Rows, err error) {

	sess := d.buildCond(cond)
	defer sess.Close()
	rows, err = sess.Rows(t)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}

	return
}

func (d *XormDao) Iterate(t interface{}, cond map[string]interface{}, f xorm.IterFunc) (err error) {

	sess := d.buildCond(cond)
	defer sess.Close()
	err = sess.Iterate(t, f)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}

	return
}

func (d *XormDao) GetMulti(t interface{}, ids ...interface{}) (err error) {
	sess := d.engine.NewSession()
	defer sess.Close()

	err = sess.In("id", ids...).Find(t)

	return
}

func (d *XormDao) Count(t interface{}, cond map[string]interface{}) (total int64, err error) {
	var (
		str   []string
		args  []interface{}
		where string
	)
	for k, v := range cond {
		k = strings.ToLower(k)
		if k == "orderby" || k == "page" || k == "pagesize" {
			continue
		}
		if k == "where" {
			where = v.(string)
			continue
		}
		str = append(str, fmt.Sprintf("`%s` = ?", k))
		args = append(args, v)
	}

	if where != "" {
		where = fmt.Sprintf("(%s) AND %s", where, strings.Join(str, " AND "))
	} else {
		where = strings.Join(str, " AND ")
	}

	sess := d.engine.NewSession()
	defer sess.Close()
	total, err = sess.Where(where, args...).Count(t)
	if err != nil {
		lastSQL, lastSQLArgs := sess.LastSQL()
		logger.Error(err, lastSQL, lastSQLArgs)
	}

	return
}

func (d *XormDao) Replace(sql string, cond map[string]interface{}) (id int64, err error) {
	var (
		str  []string
		args []interface{}
	)
	for k, v := range cond {
		k = strings.ToLower(k)
		if k == "orderby" || k == "page" || k == "pagesize" || k == "where" {
			continue
		}
		str = append(str, fmt.Sprintf("`%s` = ?", k))
		args = append(args, v)
	}

	rs, err := d.Exec(sql+strings.Join(str, ", "), args...)
	if err != nil {
		return
	}

	return rs.LastInsertId()
}

//必须确保执行Exec后，再执行ClearCache
func (d *XormDao) Exec(sqlStr string, args ...interface{}) (sql.Result, error) {
	tmp := make([]interface{}, len(args)+1)
	tmp = append(tmp, sqlStr)
	tmp = append(tmp, args...)

	return d.engine.Exec(args...)
}

func (d *XormDao) Query(args ...interface{}) ([]map[string][]byte, error) {
	return d.engine.Query(args...)
}

func (d *XormDao) QueryString(args ...interface{}) ([]map[string]string, error) {
	return d.engine.QueryString(args...)
}

func (d *XormDao) QueryInterface(args ...interface{}) ([]map[string]interface{}, error) {
	return d.engine.QueryInterface(args...)
}

func (d *XormDao) EnableCache(t interface{}) {
	if d.cacher != nil {
		d.isCache = true
		_ = d.engine.MapCacher(t, d.cacher)
	}
}

func (d *XormDao) DisableCache(t interface{}) {
	d.isCache = false
	_ = d.engine.MapCacher(t, nil)
}

//用于清理缓存
func (d *XormDao) ClearCache(t interface{}) {
	if d.isCache {
		_ = d.engine.ClearCache(t)
	}
}