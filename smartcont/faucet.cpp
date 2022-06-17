#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

void request() {
  storageVar<uint64_t> last(3);
  uint64_t cur = block::time();
  require(cur - last >= 600'000'000'000ull, "please wait for 10min");
  last = cur;
  msg::caller().transfer(1'000'000'000ull, "");
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(request);
  return 0;
}

void init() {}