use diesel::{SqliteConnection, sql_query, RunQueryDsl};
use diesel::r2d2::{ConnectionManager, Pool};

fn setup_database(conn: &mut SqliteConnection) {
    sql_query("CREATE TABLE IF NOT EXISTS files (
        absolute_file_path VARCHAR PRIMARY KEY,
        inserted_at TIMESTAMP NOT NULL,
        modified_at TIMESTAMP NOT NULL,
        expiration TIMESTAMP
    )
    ").execute(conn).unwrap();

    sql_query("CREATE TABLE IF NOT EXISTS watches (
        absolute_file_path VARCHAR PRIMARY KEY,
        created_at TIMESTAMP NOT NULL,
        default_expiration INTEGER
    )
    ").execute(conn).unwrap();

    sql_query("PRAGMA foreign_keys = ON;").execute(conn).unwrap();

    sql_query("CREATE TABLE IF NOT EXISTS files_watches (
        file_id VARCHAR NOT NULL,
        watch_id VARCHAR NOT NULL,
        PRIMARY KEY (file_id, watch_id),
        FOREIGN KEY (file_id) REFERENCES files(absolute_file_path),
        FOREIGN KEY (watch_id) REFERENCES watches(absolute_file_path)
    )
    ").execute(conn).unwrap();
}

pub fn get_pool() -> Pool<ConnectionManager<SqliteConnection>>{
    let conn_spec = std::env::var("DINHA_DATABASE_URL").expect("DINHA_DATABASE_URL");
    let manager = ConnectionManager::<SqliteConnection>::new(conn_spec);
    let pool = Pool::builder()
        .build(manager)
        .expect("Failed to create pool.");

    setup_database(&mut pool.get().unwrap());
    pool
}