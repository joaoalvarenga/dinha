diesel::table! {
    watches (absolute_file_path) {
        absolute_file_path -> Text,
        created_at -> Timestamp,
        default_expiration -> Nullable<Integer>,
    }
}