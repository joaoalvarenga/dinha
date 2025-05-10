use std::io::Stdout;

use tui::{backend::{CrosstermBackend}, Frame, layout::{Layout, Constraint, Direction}, style::{Style, Modifier, Color}, widgets::{Cell, Row, Table, Borders, Block, Paragraph}, text::Text};

use crate::app::App;

use super::{banner::BANNER};

pub fn ui(f: &mut Frame<CrosstermBackend<Stdout>>, app: &mut App) {
    let rects = Layout::default()
        .constraints([Constraint::Percentage(20), Constraint::Percentage(80)].as_ref())
        .margin(0)
        .split(f.size());

    let selected_style = Style::default().add_modifier(Modifier::REVERSED);
    let normal_style = Style::default();
    let header_cells = ["PATH", "EXPIRATION", "STATUS"]
        .iter()
        .map(|h| Cell::from(*h).style(Style::default().fg(Color::White)));
    let header = Row::new(header_cells)
        .style(normal_style)
        .height(1);
    let rows = app.items.iter().map(|item| {
        let height = item
            .iter()
            .map(|content| content.chars().filter(|c| *c == '\n').count())
            .max()
            .unwrap_or(0)
            + 1;
        let cells = item.iter().map(|c| Cell::from(c.as_str()));
        Row::new(cells).height(height as u16)
    });
    let t = Table::new(rows)
        .header(header)
        .block(Block::default().borders(Borders::ALL).title("Watching"))
        .highlight_style(selected_style)
        .widths(&[
            Constraint::Percentage(50),
            Constraint::Length(30),
            Constraint::Min(10),
        ]);
    f.render_stateful_widget(t, rects[1], &mut app.state);
    
    let create_block = || {
        Block::default()
            .borders(Borders::NONE)
    };

    let normal_style = Style::default();

    let rows = app.commands.iter().map(|item| {
        let height = item
            .iter()
            .map(|content| content.chars().filter(|c| *c == '\n').count())
            .max()
            .unwrap_or(0)
            + 1;
        let header_cell = Cell::from(item[0]).style(normal_style.add_modifier(Modifier::BOLD).fg(Color::Blue));
        let cells = vec![header_cell, Cell::from(item[1])];
        Row::new(cells).height(height as u16)
    });
    let t = Table::new(rows)
        .block(create_block())
        .highlight_style(selected_style)
        .widths(&[
            Constraint::Percentage(3),
            Constraint::Length(20),
            Constraint::Min(5),
        ]);

    let header_rects = Layout::default()
    .direction(Direction::Horizontal)
    .constraints([Constraint::Percentage(30), Constraint::Percentage(70)].as_ref())
    .margin(0)
    .split(rects[0]);
    f.render_widget(t, header_rects[1]);

    let mut top_text = Text::from(BANNER);
    top_text.patch_style(Style::default());
    let top_text = Paragraph::new(top_text)
        .style(Style::default())
        .block(Block::default());
    f.render_widget(top_text, header_rects[0]); 

    match &mut app.dialog {
        Some(dialog ) => dialog.ui(f),
        None => {}
    }
}