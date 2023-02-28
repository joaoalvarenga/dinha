use std::path::PathBuf;

use chrono::Duration;
use diesel::prelude::*;
use crate::models;

type DbError = Box<dyn std::error::Error + Send + Sync>;

pub fn find_file(conn: &mut SqliteConnection, path: &PathBuf) -> Result<Option<models::File>, DbError> {
    use crate::schema::files::dsl::*;
    let full_path = path.to_str().unwrap().to_string();

    let file = files
        .filter(absolute_file_path.eq(full_path))
        .first::<models::File>(conn)
        .optional()?;

    Ok(file)
}

pub fn create_new_file(
    conn: &mut SqliteConnection,
    path: &PathBuf
) {
    log::info!("Creating new file {}", path.display());
    use crate::schema::files::dsl::*;
    let now = chrono::prelude::Local::now();
    let dt = now + Duration::hours(1);
    let file = find_file(conn, path).unwrap();
    if file.is_some() {
        let file_data = file.unwrap();
        diesel::update(files)
        .filter(absolute_file_path.eq(file_data.absolute_file_path))
        .set(expiration.eq(Some(dt.naive_local())))
        .execute(conn).unwrap();
        return
    }

    
    let full_path = path.to_str().unwrap().to_string();
    let new_file = models::File {
        absolute_file_path: full_path,
        inserted_at: now.naive_local(),
        modified_at: now.naive_local(),
        expiration: Some(dt.naive_local())
    };

    diesel::insert_into(files).values(&new_file).execute(conn).unwrap();
}