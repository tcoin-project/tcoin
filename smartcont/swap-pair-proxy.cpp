#define NOSTART
#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

const uint64_t addr[4] = {12671449093898902ull, 11501251996043706844ull,
                          2510662708730297140ull, 9016383829188243672ull};
const uint64_t callAddr[4] = {
    14972987868389536084ull,
    8082521059089250243ull,
    8590012623063621564ull,
    14577256071116085345ull,
};

extern "C" {
entrypoint_t _start(const void *data) {
  auto start =
      syscall::loadELF(reinterpret_cast<const Address *>(callAddr), 0x1000);
  return start(addr);
}
}