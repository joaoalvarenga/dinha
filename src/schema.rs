// @generated automatically by Diesel CLI.

diesel::table! {
    files (absolute_file_path) {
        absolute_file_path -> Text,
        inserted_at -> Timestamp,
        modified_at -> Timestamp,
        expiration -> Nullable<Timestamp>,
    }
}
