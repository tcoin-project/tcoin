#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

void request() {
  Address key(3), val;
  storage::load(key, val.s);
  char *t = val.s;
  uint64_t last;
  deserialize(t, last);
  uint64_t cur = block::time();
  require(cur - last >= 600'000'000'000ull, "please wait for 10min");
  t = val.s;
  serialize(t, cur);
  storage::store(key, val);
  msg::caller().transfer(1'000'000'000ull, "");
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(request);
  return 0;
}

void init() {}