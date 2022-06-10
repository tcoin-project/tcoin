#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "syscall.h"
#include "tcoin.h"

const uint8_t code[8] = {1, 0, 2, 43, 221, 53, 124, 21};

uint64_t test() {
  Address x =
      syscall::create(reinterpret_cast<const char *>(code), 8,
                      CREATE_INIT | CREATE_TRIMELF | CREATE_USENONCE, 123);
  return x.balance();
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(test);
  return 0;
}

void init(void *initData) {}