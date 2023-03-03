// #[macro_use]
// extern crate diesel;

use diesel::{SqliteConnection, r2d2, sql_query, RunQueryDsl};
use diesel::r2d2::ConnectionManager;
use notify::{watcher, DebouncedEvent};
use std::collections::{VecDeque};
use std::sync::{Arc, Mutex};
use std::sync::mpsc::channel;
use std::time::Duration;

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
        default_expiration TIMESTAMP
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
    
    let event_queue: Arc<Mutex<VecDeque<DebouncedEvent>>> = Arc::new(Mutex::new(VecDeque::new()));
    let file_event_pool = pool.clone();
    let file_event_queue = event_queue.clone();
    let file_event_thread = std::thread::spawn(move || dinha::file_event::service::get_runner(file_event_pool, file_event_queue));
    

    // Create a channel to receive the events.
    let (tx, rx) = channel();

    let watcher = Arc::new(Mutex::new(watcher(tx, Duration::from_secs(1)).unwrap()));

    let watch_pool = pool.clone();
    let thread_watcher = std::thread::spawn(move || dinha::watch::service::get_runner(watch_pool, watcher));

    loop {
        match rx.recv() {
           Ok(event) => event_queue.lock().unwrap().push_back(event),
           Err(e) => println!("watch error: {:?}", e),
        }
    }
    file_event_thread.join().unwrap();
    thread_watcher.join().unwrap();
}