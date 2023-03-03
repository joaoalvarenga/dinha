use std::{collections::HashSet, sync::{Arc, Mutex}};
use diesel::{prelude::*, r2d2::{Pool, ConnectionManager}};
use notify::{RecursiveMode, Watcher, INotifyWatcher};

use super::model;

type DbError = Box<dyn std::error::Error + Send + Sync>;

pub fn list_watches(
    conn: &mut SqliteConnection
) -> Result<Vec<model::Watch>, DbError> {
    use super::schema::watches::dsl::*;
    let all_watches = watches.load::<model::Watch>(conn)?;
    Ok(all_watches)
}

pub fn find_watch(conn: &mut SqliteConnection, path: String) -> Result<Option<model::Watch>, DbError> {
    use super::schema::watches::dsl::*;

    let watch = watches
        .filter(absolute_file_path.eq(path))
        .first::<model::Watch>(conn)
        .optional()?;

    Ok(watch)
}

pub fn upsert_watch(
    conn: &mut SqliteConnection,
    path: String,
    expiration: Option<i32>
) {
    use super::schema::watches::dsl::*;
    let now = chrono::prelude::Local::now();
    let watch = find_watch(conn, path.clone()).unwrap();
    match watch {
        Some(w) => {
            diesel::update(watches)
                .filter(absolute_file_path.eq(w.absolute_file_path))
                .set(default_expiration.eq(&expiration)).execute(conn).unwrap();
        },
        None => {
            let new_watch = model::Watch {
                absolute_file_path: path,
                created_at: now.naive_local(),
                default_expiration: expiration
            };
            diesel::insert_into(watches).values(&new_watch).execute(conn).unwrap();
        }
    };
}

pub fn delete_watch(conn: &mut SqliteConnection, path: String) {
    use super::schema::watches::dsl::*;

    diesel::delete(watches).filter(absolute_file_path.eq(path)).execute(conn).unwrap();
}

pub fn get_runner(
    pool: Pool<ConnectionManager<SqliteConnection>>,
    watcher: Arc<Mutex<INotifyWatcher>>
) -> fn() {
    {
        let mut all_watches: HashSet<String> = HashSet::new();
        let mut conn = pool.get().unwrap();
        let watches = list_watches(&mut conn).unwrap();
        for watch in &watches {
            all_watches.insert(watch.absolute_file_path.clone());
            watcher.lock().unwrap().watch(watch.absolute_file_path.clone(), RecursiveMode::Recursive).unwrap();
        }
        println!("Watching: {:?}", all_watches);

        loop {
            let some_time = std::time::Duration::from_millis(50);
            let watches = list_watches(&mut conn).unwrap();
            let mut new_watches: Vec<model::Watch> = Vec::new();
            let mut all_new_watches: HashSet<String> = HashSet::new();
            for watch in &watches {
                all_new_watches.insert(watch.absolute_file_path.clone());
                if all_watches.contains(watch.absolute_file_path.as_str()) {
                    continue;
                }
                new_watches.push(watch.clone());
                all_watches.insert(watch.absolute_file_path.clone());
                watcher.lock().unwrap().watch(watch.absolute_file_path.clone(), RecursiveMode::Recursive).unwrap();
            }
            
            let tmp_watches = all_watches.clone();
            let watches_to_remove: HashSet<_> = tmp_watches.difference(&all_new_watches).collect();
            for w in &watches_to_remove {
                watcher.lock().unwrap().unwatch(w).unwrap();
                let data = w.clone();
                all_watches.remove(data);
            }
            if new_watches.len() > 0 || watches_to_remove.len() > 0 {
                println!("Watching: {:?}", all_watches);
            }
            std::thread::sleep(some_time);
        }
    }
}