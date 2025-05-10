use crate::{DbPool, watch::service::delete_watch};

use super::dialog::Dialog;


pub fn create(pool: DbPool, path: &str) -> Dialog<String> {

    let success_callback = |pool: DbPool, data: String| {
        let mut conn = pool.get().unwrap();
        delete_watch(&mut conn, data);
    };
    let content = format!("Do you really want to unwatch {} folder?", path);
    Dialog::new(pool.clone(), String::from("Unwatch"), content, path.to_string()).success_callback(success_callback)
}