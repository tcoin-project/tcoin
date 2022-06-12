#ifndef _TCOIN_H
#define _TCOIN_H

#include "stdlib.h"
#include "syscall.h"

static constexpr size_t ADDR_LEN = 32;

struct Address {
  char s[ADDR_LEN];
  Address() { memsetAligned(s, 0, ADDR_LEN); }
  Address(const Address &x) { memcpyAligned(s, x.s, ADDR_LEN); }
  Address(uint64_t x) {
    *reinterpret_cast<uint64_t *>(s) = x;
    for (size_t i = 8; i < ADDR_LEN; i++)
      *reinterpret_cast<uint64_t *>(s + i) = 0;
  }
  operator const char *() const { return s; }
  uint64_t balance();
  bool transfer(uint64_t value, const char *msg);
};

struct Contract {
  void *(*call)(uint64_t, const void *);
  void *operator()(uint64_t callId, const void *callData) {
    return call(callId, callData);
  }
};

void serialize(char *&ptr, uint64_t x);
void deserialize(char *&ptr, uint64_t &x);
constexpr size_t serializeLen(uint64_t x) { return sizeof(uint64_t); }
void serialize(char *&ptr, const Address &x);
void deserialize(char *&ptr, Address &x);
constexpr size_t serializeLen(const Address &x) { return ADDR_LEN; }

#define PRIVATE_DATA __attribute__((section(".private_data")))
#define SHARED_DATA __attribute__((section(".shared_data")))
#define INIT_CODE __attribute__((section(".init_code")))

extern "C" {
typedef const void *(*entrypoint_t)(uint32_t callId, void *callData);
typedef entrypoint_t (*start_t)();
entrypoint_t regularStart();
void init(void *initData) INIT_CODE;
start_t _start(void *initData) INIT_CODE;
const void *entrypoint(uint32_t callId, void *callData);
};

namespace syscall {
constexpr uint64_t addr(int id) { return ((-1ull) >> 1) + 1 - id * 4; }
const auto balance = reinterpret_cast<uint64_t (*)(const Address *addr)>(
    syscall::addr(SYSCALL_BALANCE));
const auto markJumpDest =
    reinterpret_cast<void (*)(void *addr)>(syscall::addr(SYSCALL_JUMPDEST));
const auto protectedCall = reinterpret_cast<const void *(
        *)(void *(call)(uint64_t, void *), uint64_t a1, void *a2,
           uint64_t value, uint64_t gasLimit, bool *success, char *errorMsg)>(
    syscall::addr(SYSCALL_PROTECTED_CALL));
const auto transfer = reinterpret_cast<void (*)(
    const Address *addr, uint64_t value, const char *msg, size_t msgLen)>(
    syscall::addr(SYSCALL_TRANSFER));
const auto create = reinterpret_cast<Address (*)(
    const char *code, size_t len, uint64_t flags, uint64_t nonce)>(
    syscall::addr(SYSCALL_CREATE));
const auto loadELF =
    reinterpret_cast<start_t (*)(const Address *addr, size_t offset)>(
        syscall::addr(SYSCALL_LOAD_ELF));
} // namespace syscall

namespace msg {
const auto origin =
    reinterpret_cast<Address (*)()>(syscall::addr(SYSCALL_ORIGIN));
const auto caller =
    reinterpret_cast<Address (*)()>(syscall::addr(SYSCALL_CALLER));
const auto value =
    reinterpret_cast<uint64_t (*)()>(syscall::addr(SYSCALL_CALLVALUE));
} // namespace msg
namespace storage {
const auto store =
    reinterpret_cast<void (*)(const char *key, const char *value)>(
        syscall::addr(SYSCALL_STORAGE_STORE));
const auto load = reinterpret_cast<void (*)(const char *key, char *value)>(
    syscall::addr(SYSCALL_STORAGE_LOAD));
} // namespace storage
namespace block {
const auto time = reinterpret_cast<uint64_t (*)()>(syscall::addr(SYSCALL_TIME));
const auto miner =
    reinterpret_cast<Address (*)()>(syscall::addr(SYSCALL_MINER));
const auto number =
    reinterpret_cast<uint64_t (*)()>(syscall::addr(SYSCALL_BLOCK_NUMBER));
const auto difficulty =
    reinterpret_cast<Address (*)()>(syscall::addr(SYSCALL_DIFFICULTY));
const auto chainid =
    reinterpret_cast<uint16_t (*)()>(syscall::addr(SYSCALL_CHAINID));
} // namespace block
namespace crypto {
const auto sha256 =
    reinterpret_cast<void (*)(const char *key, size_t len, char *resHash)>(
        syscall::addr(SYSCALL_SHA256));
const auto ed25519Verify = reinterpret_cast<bool (*)(
    const char *msg, size_t len, const char *pubkey, const char *sig)>(
    syscall::addr(SYSCALL_ED25519_VERIFY));
} // namespace crypto
const auto self = reinterpret_cast<Address (*)()>(syscall::addr(SYSCALL_SELF));
const auto loadContract = reinterpret_cast<Contract (*)(const Address *addr)>(
    syscall::addr(SYSCALL_LOAD_CONTRACT));
const auto revert =
    reinterpret_cast<void (*)(const char *)>(syscall::addr(SYSCALL_REVERT));
const auto gasleft =
    reinterpret_cast<uint64_t (*)()>(syscall::addr(SYSCALL_GAS));

void require(bool cond, const char *revertMsg);

#ifndef NO_MALLOC
#include <new>
void *malloc(size_t n);
void *mallocShared(size_t n);
template <typename T> const T *asSharedPtr(const T &x) {
  T *ptr = reinterpret_cast<T *>(mallocShared(sizeof(x)));
  new (ptr) T(x);
  return ptr;
}
#endif

namespace __selector {
constexpr uint32_t fnv1a_32(const char *s, size_t count) {
  return ((count ? fnv1a_32(s, count - 1) : 2166136261u) ^ s[count]) *
         16777619u;
}
constexpr size_t strlen(const char *s) { return *s ? 0 : strlen(s + 1) + 1; }
constexpr size_t findNamespace(const char *s) {
  return s[0] && s[1]
             ? (s[0] == ':' && s[1] == ':' ? 0 : findNamespace(s + 1) + 1)
             : size_t(1ull << 63);
}
} // namespace __selector
constexpr uint32_t selector(const char *s) {
  using namespace __selector;
  return findNamespace(s) >= 0 ? selector(s + findNamespace(s) + 2)
                               : fnv1a_32(s, __selector::strlen(s));
}

template <typename T> struct __receiveCall;
template <typename resType> struct __receiveCall<resType()> {
  static resType call(resType (*func)(), void *arg) { return func(); }
};
template <typename resType, typename argType>
struct __receiveCall<resType(argType)> {
  static resType call(resType (*func)(argType), void *arg) {
    return func(reinterpret_cast<argType>(arg));
  }
};
template <typename V, typename... L> struct __receiveCallMulti {
  template <typename X, typename... R> struct loader {
    static V load(V (*func)(L..., X, R...), void **a, L... l) {
      return __receiveCallMulti<V, L..., X>::template loader<R...>::load(
          func, a + 1, l..., reinterpret_cast<X>(*a));
    }
  };
  template <typename X> struct loader<X> {
    static V load(V (*func)(L..., X), void **a, L... l) {
      return func(l..., reinterpret_cast<X>(*a));
    }
  };
};
template <typename resType, typename... argType>
struct __receiveCall<resType(argType...)> {
  static resType call(resType (*func)(argType...), void *arg) {
    return __receiveCallMulti<resType>::template loader<argType...>::load(
        func, reinterpret_cast<void **>(arg));
  }
};
template <typename T> struct __receiveCast;
template <typename resType, typename callType>
struct __receiveCast<resType(callType, void *)> {
  static const void *cast(resType (*call)(callType, void *), callType func,
                          void *arg) {
    return reinterpret_cast<const void *>(call(func, arg));
  }
};
template <typename callType> struct __receiveCast<void(callType, void *)> {
  static const void *cast(void (*call)(callType, void *), callType func,
                          void *arg) {
    call(func, arg);
    return reinterpret_cast<const void *>(0);
  }
};

#define export(func)                                                           \
  if (callId == selector(#func)) {                                             \
    return __receiveCast<__typeof(__receiveCall<__typeof(func)>::call)>::cast( \
        __receiveCall<__typeof(func)>::call, func, callData);                  \
  }

template <typename T> struct __resCast {
  static T cast(void *x) { return reinterpret_cast<T>(x); }
};
template <> struct __resCast<void> {
  static void cast(void *x) {}
};

template <typename T> struct __makeCall;
template <typename contractType, typename resType>
struct __makeCall<resType (contractType::*)()> {
  typedef resType resType_;
  static resType call(contractType *_this, uint64_t callId) {
    void *res = _this->call(callId, 0);
    return __resCast<resType>::cast(res);
  }
};
template <typename contractType, typename resType, typename argType>
struct __makeCall<resType (contractType::*)(argType)> {
  typedef resType resType_;
  typedef argType argType_;
  static resType call(contractType *_this, uint64_t callId, argType arg) {
    void *res = _this->call(callId, reinterpret_cast<const void *>(arg));
    return __resCast<resType>::cast(res);
  }
};
template <typename C, typename... L> struct __makeCallMulti {
  template <typename X, typename... R> struct storer {
    static int nargs() {
      return __makeCallMulti<C, L..., X>::template storer<R...>::nargs() + 1;
    }
    static void store(const void **a, X x, R... r) {
      *a = reinterpret_cast<const void *>(x);
      __makeCallMulti<C, L..., X>::template storer<R...>::store(a + 1, r...);
    }
  };
  template <typename X> struct storer<X> {
    static int nargs() { return 1; }
    static void store(const void **a, X x) {
      *a = reinterpret_cast<const void *>(x);
    }
  };
  template <const int n, typename X, typename... R> struct argGetter {
    typedef
        typename __makeCallMulti<C, L..., X>::template argGetter<n - 1, R...>::T
            T;
  };
  template <typename X, typename... R> struct argGetter<0, X, R...> {
    typedef X T;
  };
};
template <typename contractType, typename resType, typename... argType>
struct __makeCall<resType (contractType::*)(argType...)> {
  typedef resType resType_;
  template <const int n> struct argType_ {
    typedef typename __makeCallMulti<contractType>::template argGetter<
        n, argType...>::T T;
  };
  static resType call(contractType *_this, uint64_t callId, argType... arg) {
    int nargs =
        __makeCallMulti<contractType>::template storer<argType...>::nargs();
    const void **ptr =
        reinterpret_cast<const void **>(mallocShared(nargs * sizeof(void *)));

    __makeCallMulti<contractType>::template storer<argType...>::store(ptr,
                                                                      arg...);
    void *res = _this->call(callId, reinterpret_cast<const void *>(ptr));
    return __resCast<resType>::cast(res);
  }
};

#define impl0(func)                                                            \
  __makeCall<__typeof(&func)>::resType_ func() {                               \
    return __makeCall<__typeof(&func)>::call(this, selector(#func));           \
  }
#define impl1(func)                                                            \
  __makeCall<__typeof(&func)>::resType_ func(                                  \
      __makeCall<__typeof(&func)>::argType_ a1) {                              \
    return __makeCall<__typeof(&func)>::call(this, selector(#func), a1);       \
  }
// https://www.scs.stanford.edu/~dm/blog/va-opt.html
#define __parens ()
#define __expand(...) __expand4(__expand4(__expand4(__expand4(__VA_ARGS__))))
#define __expand4(...) __expand3(__expand3(__expand3(__expand3(__VA_ARGS__))))
#define __expand3(...) __expand2(__expand2(__expand2(__expand2(__VA_ARGS__))))
#define __expand2(...) __expand1(__expand1(__expand1(__expand1(__VA_ARGS__))))
#define __expand1(...) __VA_ARGS__
#define __implMulti_arg(func, n)                                               \
  __makeCall<__typeof(&func)>::template argType_<n>::T a##n
#define __implMulti_call(func, n) a##n
#define __implMulti_forEach(macro, func, a1, ...)                              \
  macro(func, a1) __VA_OPT__(                                                  \
      __expand(__implMulti_forEachHelper(macro, func, __VA_ARGS__)))
#define __implMulti_forEachHelper(macro, func, a1, ...)                        \
  , macro(func, a1) __VA_OPT__(                                                \
        __implMulti_forEachAgain __parens(macro, func, __VA_ARGS__))
#define __implMulti_forEachAgain() __implMulti_forEachHelper
#define implN(n, func)                                                         \
  __makeCall<__typeof(&func)>::resType_ func(                                  \
      __implMulti_forEach(__implMulti_arg, func, 0, __seq##n)) {               \
    return __makeCall<__typeof(&func)>::call(                                  \
        this, selector(#func),                                                 \
        __implMulti_forEach(__implMulti_call, func, 0, __seq##n));             \
  }
#define __seq2 1
#define __seq3 __seq2, 2
#define __seq4 __seq3, 3
#define __seq5 __seq4, 4
#define __seq6 __seq5, 5
#define __seq7 __seq6, 6
#define __seq8 __seq7, 7
#define __seq9 __seq8, 8
#define __seq10 __seq9, 9
#define __seq11 __seq10, 10
#define __seq12 __seq11, 11
#define __seq13 __seq12, 12
#define __seq14 __seq13, 13
#define __seq15 __seq14, 14
#define __seq16 __seq15, 15
#define __seq17 __seq16, 16
#define __seq18 __seq17, 17
#define __seq19 __seq18, 18
#define __seq20 __seq19, 19
#define __seq21 __seq20, 20
#define __seq22 __seq21, 21
#define __seq23 __seq22, 22
#define __seq24 __seq23, 23
#define __seq25 __seq24, 24
#define __seq26 __seq25, 25
#define __seq27 __seq26, 26
#define __seq28 __seq27, 27
#define __seq29 __seq28, 28
#define __seq30 __seq29, 29
#define __seq31 __seq30, 30
#define __seq32 __seq31, 31
#define impl2(func) implN(2, func)
#define impl3(func) implN(3, func)
#define impl4(func) implN(4, func)
#define impl5(func) implN(5, func)
#define impl6(func) implN(6, func)
#define impl7(func) implN(7, func)
#define impl8(func) implN(8, func)
#define impl9(func) implN(9, func)
#define impl10(func) implN(10, func)
#define impl11(func) implN(11, func)
#define impl12(func) implN(12, func)
#define impl13(func) implN(13, func)
#define impl14(func) implN(14, func)
#define impl15(func) implN(15, func)
#define impl16(func) implN(16, func)
#define impl17(func) implN(17, func)
#define impl18(func) implN(18, func)
#define impl19(func) implN(19, func)
#define impl20(func) implN(20, func)
#define impl21(func) implN(21, func)
#define impl22(func) implN(22, func)
#define impl23(func) implN(23, func)
#define impl24(func) implN(24, func)
#define impl25(func) implN(25, func)
#define impl26(func) implN(26, func)
#define impl27(func) implN(27, func)
#define impl28(func) implN(28, func)
#define impl29(func) implN(29, func)
#define impl30(func) implN(30, func)
#define impl31(func) implN(31, func)
#define impl32(func) implN(32, func)

#endif