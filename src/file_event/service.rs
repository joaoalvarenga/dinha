use std::{sync::{Arc, Mutex}, collections::VecDeque};

use diesel::{r2d2::{Pool, ConnectionManager}, SqliteConnection};
use notify::DebouncedEvent;

use crate::file;

fn process_event(pool: Pool<ConnectionManager<SqliteConnection>>, event: DebouncedEvent) {
    let mut conn = pool.get().unwrap();
    match event {
        DebouncedEvent::NoticeWrite(path) => file::service::create_new_file(&mut conn, &path),
        DebouncedEvent::Write(path) => file::service::create_new_file(&mut conn, &path),
        DebouncedEvent::Create(path) => file::service::create_new_file(&mut conn, &path),
        _ => println!("default")
    }
}

pub fn get_runner(
    pool: Pool<ConnectionManager<SqliteConnection>>, 
    event_queue: Arc<Mutex<VecDeque<DebouncedEvent>>>) -> fn() {
    {
        let some_time = std::time::Duration::from_millis(50);
        loop {
            let event = event_queue.lock().unwrap().pop_front();
            if event.is_some() {
                process_event(pool.clone(), event.unwrap());
            }
            std::thread::sleep(some_time);
        }
    }
}