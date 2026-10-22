fn main() {
    switch a {
        1 => { return 1; }
        2 => { return 2; },
    }
    var a = switch b {
        1 => 2,
        2 => 3,
        _ => 4,
    };
    return switch c {
        "hello" => "world",
        "world" => {
            >> try list[0];
        }
    }
    return switch try args[0] {
        "add" => {
            var title = try args[1] else {
                return Err("you need to provide a title");
            };
            >> Ok(Action.Add(title=title));
        }
        "list" => Ok(Action.List),
        "exit" => Ok(Action.Exit),
        None => Err("Command missing"),
        _ => Err("invalid command"),
    }
}
