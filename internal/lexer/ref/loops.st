for i in 0..10 {
    print(i)
}

loop {
  var shouldStop = shouldStop()
  if shouldStop {
      break
  } else {
      continue
  }
}
