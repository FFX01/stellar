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
    };
}
