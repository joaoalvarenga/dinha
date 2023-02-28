
use serde::{Deserialize, Serialize};

use crate::schema::files;

#[derive(Debug, Clone, Serialize, Deserialize, diesel::Queryable, diesel::Insertable)]
pub struct File {
    pub absolute_file_path: String,
    pub inserted_at: chrono::NaiveDateTime,
    pub modified_at: chrono::NaiveDateTime,
    pub expiration: Option<chrono::NaiveDateTime>,
}