// The versions odm-libtorrent reports.
#pragma once

#ifndef ODM_LIBTORRENT_VERSION
#define ODM_LIBTORRENT_VERSION "dev"
#endif

namespace odm {

// kProtocol is the version of the protocol ODM speaks with odm-libtorrent
// (PROTOCOL.md). It changes when a change would break an ODM that knows the
// previous one, and it is part of the release tag, odm-libtorrent-p<N>-…, so
// that an ODM only downloads a program it can talk to.
constexpr int kProtocol = 1;

}  // namespace odm
