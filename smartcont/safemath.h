#include "stdlib.h"
#include "tcoin.h"

static bool checkAdd(uint64_t a, uint64_t b) { return (a + b) >= a; }
static bool checkSub(uint64_t a, uint64_t b) { return (a - b) <= a; }