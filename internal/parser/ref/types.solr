// type Todo object {
  //   pub title: str,
    // pub status: Status,
// }

// type Status enum {
    // InProgress,
    // Done,
// }

// type Shape enum {
    // Rectangle(h: int, w: int),
    // Circle(r: int),
// }

// type Container[T] object {
    // item: T,
// }

// type Stringable interface {
    // fn string() -> str
    // fn write() -> void
// }

type Handler = fn(Event) -> Maybe[Action];
type Handler2[T] = fn(e: Event, v: T) -> Maybe[Action[T]];

pub type Store[Tkey, Tvalue] object {
    pub key: Tkey,
    pub value: Tvalue,
}

impl String for Store {
    pub fn string(self: Store) -> str {
        return self.key + ": " + self.value;
    }
    fn write(self: Store, s: str) {
        var cut: []str = strings.cut(s.trim(), ":");
        self.k = try cut[0] else self.k;
        self.v = try cut[1] else self.v; // need to implement index access infix
        self.k = "a";
    }
}

type UserId = int;
type Handler[T] = fn(e: Event) -> T;
type Alias = io.http.Reader[io.http.Request];
type Status enum {
    InProgress,
    Done,
}
