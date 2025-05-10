use crossterm::{
    event::{DisableMouseCapture, EnableMouseCapture},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use dinha::app::{App, run_app};
use std::{error::Error, io, time::{Duration}};
use tui::{
    backend::{CrosstermBackend}, Terminal
};




fn main() -> Result<(), Box<dyn Error>> {
    dotenv::dotenv().ok();
    let pool = dinha::db::get_pool();
    // setup terminal
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    // create app and run it
    let app = App::new();
    let tick_rate = Duration::from_millis(250);
    let res = run_app(pool, &mut terminal, app, tick_rate);


    // restore terminal
    disable_raw_mode()?;
    execute!(
        terminal.backend_mut(),
        LeaveAlternateScreen,
        DisableMouseCapture
    )?;
    terminal.show_cursor()?;

    if let Err(err) = res {
        println!("{:?}", err)
    }

    Ok(())
}