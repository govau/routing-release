package testsupport

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/go-db-helpers/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type DBConnectionInfo struct {
	Type           string
	Hostname       string
	Port           string
	Username       string
	Password       string
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

type TestDatabase struct {
	Name     string
	ConnInfo *DBConnectionInfo
}

func encodeAsQueryString(values map[string]interface{}) string {
	queryValues := url.Values{}
	for k, v := range values {
		queryValues.Add(k, fmt.Sprintf("%v", v))
	}
	return queryValues.Encode()
}

func (d *TestDatabase) mysqlConnectionString() string {
	queryStringValues := map[string]interface{}{
		"parseTime": true,
	}
	if d.ConnInfo.ConnectTimeout != 0 {
		queryStringValues["timeout"] = d.ConnInfo.ConnectTimeout
	}
	if d.ConnInfo.ReadTimeout != 0 {
		queryStringValues["readTimeout"] = d.ConnInfo.ReadTimeout
	}
	if d.ConnInfo.WriteTimeout != 0 {
		queryStringValues["writeTimeout"] = d.ConnInfo.WriteTimeout
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		d.ConnInfo.Username, d.ConnInfo.Password, d.ConnInfo.Hostname, d.ConnInfo.Port, d.Name,
		encodeAsQueryString(queryStringValues),
	)
}

func (d *TestDatabase) postgresConnectionString() string {
	asMilliseconds := func(dur time.Duration) int64 {
		return dur.Nanoseconds() / 1000 / 1000
	}
	queryStringValues := map[string]interface{}{
		"sslmode": "disable",
	}
	if d.ConnInfo.ConnectTimeout != 0 {
		queryStringValues["connect_timeout"] = asMilliseconds(d.ConnInfo.ConnectTimeout)
	}
	if d.ConnInfo.ReadTimeout != 0 {
		queryStringValues["read_timeout"] = asMilliseconds(d.ConnInfo.ReadTimeout)
	}
	if d.ConnInfo.WriteTimeout != 0 {
		queryStringValues["write_timeout"] = asMilliseconds(d.ConnInfo.WriteTimeout)
	}

	// queryStringValues["statement_timeout"] = asMilliseconds(900 * time.Millisecond)

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?%s",
		d.ConnInfo.Username, d.ConnInfo.Password, d.ConnInfo.Hostname, d.ConnInfo.Port, d.Name,
		encodeAsQueryString(queryStringValues),
	)
}

func (d *TestDatabase) DBConfig() db.Config {
	var connectionString string
	if d.ConnInfo.Type == "mysql" {
		connectionString = d.mysqlConnectionString()
	} else if d.ConnInfo.Type == "postgres" {
		connectionString = d.postgresConnectionString()
	} else {
		connectionString = fmt.Sprintf("some unsupported db type connection string: %s\n", d.ConnInfo.Type)
	}

	return db.Config{
		Type:             d.ConnInfo.Type,
		ConnectionString: connectionString,
	}
}

func (d *TestDatabase) Destroy() {
	d.ConnInfo.RemoveDatabase(d)
}

func (c *DBConnectionInfo) CreateDatabase(dbName string) *TestDatabase {
	testDB := &TestDatabase{Name: dbName, ConnInfo: c}
	_, err := c.execSQL(fmt.Sprintf("CREATE DATABASE %s", dbName))
	Expect(err).NotTo(HaveOccurred())
	return testDB
}

func (c *DBConnectionInfo) RemoveDatabase(db *TestDatabase) {
	_, err := c.execSQL(fmt.Sprintf("DROP DATABASE %s", db.Name))
	Expect(err).NotTo(HaveOccurred())
}

func (c *DBConnectionInfo) execSQL(sqlCommand string) (string, error) {
	var cmd *exec.Cmd

	if c.Type == "mysql" {
		cmd = exec.Command("mysql",
			"-h", c.Hostname,
			"-P", c.Port,
			"-u", c.Username,
			"-e", sqlCommand)
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+c.Password)
	} else if c.Type == "postgres" {
		cmd = exec.Command("psql",
			"-h", c.Hostname,
			"-p", c.Port,
			"-U", c.Username,
			"-c", sqlCommand)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+c.Password)
	} else {
		panic("unsupported database type: " + c.Type)
	}

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, "9s").Should(gexec.Exit())
	if session.ExitCode() != 0 {
		return "", fmt.Errorf("unexpected exit code: %d", session.ExitCode())
	}
	return string(session.Out.Contents()), nil
}

const DefaultDBTimeout = 5 * time.Second

func GetPostgresDBConnectionInfo() *DBConnectionInfo {
	return &DBConnectionInfo{
		Type:           "postgres",
		Hostname:       "127.0.0.1",
		Port:           "5432",
		Username:       "postgres",
		Password:       "",
		ConnectTimeout: DefaultDBTimeout,
		ReadTimeout:    DefaultDBTimeout,
		WriteTimeout:   DefaultDBTimeout,
	}
}

func GetMySQLDBConnectionInfo() *DBConnectionInfo {
	return &DBConnectionInfo{
		Type:           "mysql",
		Hostname:       "127.0.0.1",
		Port:           "3306",
		Username:       "root",
		Password:       "password",
		ConnectTimeout: DefaultDBTimeout,
		ReadTimeout:    DefaultDBTimeout,
		WriteTimeout:   DefaultDBTimeout,
	}
}

func GetDBConnectionInfo() *DBConnectionInfo {
	switch os.Getenv("DB") {
	case "mysql":
		return GetMySQLDBConnectionInfo()
	case "postgres":
		return GetPostgresDBConnectionInfo()
	default:
		panic("unable to determine database to use.  Set environment variable DB")
	}
}
