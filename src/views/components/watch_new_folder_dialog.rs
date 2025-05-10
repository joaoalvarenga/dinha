use std::collections::HashMap;

use diesel::SqliteConnection;

use crate::{DbPool, watch::service::upsert_watch};

use super::form_dialog::{Field, FormDialog};
use lazy_static::lazy_static;
use regex::Regex;

fn add_watch(conn: &mut SqliteConnection, fields: &Vec<Field>) {
    lazy_static! {
        static ref DURATION: Regex = Regex::new("([0-9]+)([dDHh])").unwrap();
    }
    lazy_static! {
        static ref MULTIPLIER: HashMap<&'static str, i32> = HashMap::from([("d", 86400), ("h", 3600)]);
    }
    let mut map: HashMap<&str, &Field> = HashMap::new();
    for field in fields.iter() {
        let f = field.clone();
        map.insert(f.id.as_str(), field);
    }
    let expiration_str = map.get("expiration").unwrap().value.to_lowercase();
    let expiration_str = expiration_str.as_str();
    let mut expiration: Option<i32> = None;
    
    if DURATION.is_match(expiration_str) {
        let caps = DURATION.captures(expiration_str).unwrap();
        let parsed_number = caps.get(1).unwrap().as_str();
        let parsed_multiplier = caps.get(2).unwrap().as_str();
        let number: i32 = parsed_number.parse().unwrap();
        let multiplier = *MULTIPLIER.get(parsed_multiplier).unwrap();
        let total = number * multiplier;
        expiration = Some(total);
    }

    let path = map.get("path").unwrap().value.clone();

    upsert_watch(conn, path, expiration);

}

pub fn create(pool: DbPool, path: String) -> FormDialog {

    let success_callback = |pool: DbPool, fields: &Vec<Field>| {
        let mut conn = pool.get().unwrap();
        add_watch(&mut conn, fields);
    };
    let fields = vec![
        Field {
            id: String::from("path"),
            label: String::from("Path (e.g. /home/user):"),
            value: path.clone()
        },
        Field {
            id: String::from("expiration"),
            label: String::from("Expiration (e.g. 30d, 24h...):"),
            value: String::new()
        },
    ];
    FormDialog::new(pool.clone()).title(String::from("New Watch")).fields(fields).success_callback(success_callback)
}