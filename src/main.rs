// #[macro_use]
// extern crate diesel;

use actions::create_new_file;
use diesel::{SqliteConnection, r2d2, sql_query, RunQueryDsl};
use diesel::r2d2::ConnectionManager;
use notify::{Watcher, RecursiveMode, watcher, DebouncedEvent};
use std::sync::mpsc::channel;
use std::time::Duration;

mod schema;
mod models;
mod actions;

type DbPool = r2d2::Pool<ConnectionManager<SqliteConnection>>;

fn process_event(pool: DbPool, event: DebouncedEvent) {
    let mut conn = pool.get().unwrap();
    match event {
        DebouncedEvent::NoticeWrite(path) => create_new_file(&mut conn, &path),
        DebouncedEvent::Write(path) => create_new_file(&mut conn, &path),
        DebouncedEvent::Create(path) => create_new_file(&mut conn, &path),
        _ => println!("default")
    }
}

fn setup_database(conn: &mut SqliteConnection) {
    sql_query("CREATE TABLE IF NOT EXISTS files (
        absolute_file_path VARCHAR PRIMARY KEY,
        inserted_at TIMESTAMP NOT NULL,
        modified_at TIMESTAMP NOT NULL,
        expiration TIMESTAMP
    )
    ").execute(conn).unwrap();
}

fn main() {
    dotenv::dotenv().ok();
    env_logger::init_from_env(env_logger::Env::new().default_filter_or("info"));
    let conn_spec = std::env::var("DINHA_DATABASE_URL").expect("DINHA_DATABASE_URL");
    let manager = ConnectionManager::<SqliteConnection>::new(conn_spec);
    let pool = r2d2::Pool::builder()
        .build(manager)
        .expect("Failed to create pool.");

    setup_database(&mut pool.get().unwrap());
    log::info!("Started database");
    

    // Create a channel to receive the events.
    let (tx, rx) = channel();

    // Create a watcher object, delivering debounced events.
    // The notification back-end is selected based on the platform.
    let mut watcher = watcher(tx, Duration::from_secs(10)).unwrap();

    // Add a path to be watched. All files and directories at that path and
    // below will be monitored for changes.
    watcher.watch("/home/joao/Downloads", RecursiveMode::Recursive).unwrap();

    loop {
        match rx.recv() {
           Ok(event) => {
            process_event(pool.clone(), event)
           },
           Err(e) => println!("watch error: {:?}", e),
        }
    }
}