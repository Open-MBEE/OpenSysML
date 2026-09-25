//! Diagnostic helper used by private-service lifecycle tests.

fn main() {
    let connection = match opensysml::Connection::private() {
        Ok(connection) => connection,
        Err(error) => {
            eprintln!("{error}");
            std::process::exit(1);
        }
    };
    if let Some(pid) = connection.private_service_pid() {
        println!("{pid}");
    }
    std::thread::sleep(std::time::Duration::from_secs(60));
}
