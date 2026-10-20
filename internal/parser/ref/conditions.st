if a {
    return 1;
} else {
    return 2;
}

var a = if f() 1 else 2;
var a = if f() {
    log.info("test");
    >> 1;
} else {
    log.info("untest");
    >> 2;
};
