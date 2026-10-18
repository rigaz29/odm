#include "helper.hpp"

#include "util.hpp"
#include "version.hpp"

#include <libtorrent/add_torrent_params.hpp>
#include <libtorrent/alert_types.hpp>
#include <libtorrent/announce_entry.hpp>
#include <libtorrent/load_torrent.hpp>
#include <libtorrent/magnet_uri.hpp>
#include <libtorrent/operations.hpp>
#include <libtorrent/portmap.hpp>
#include <libtorrent/read_resume_data.hpp>
#include <libtorrent/session_params.hpp>
#include <libtorrent/settings_pack.hpp>
#include <libtorrent/socket.hpp>
#include <libtorrent/socket_type.hpp>
#include <libtorrent/span.hpp>
#include <libtorrent/torrent_flags.hpp>
#include <libtorrent/torrent_info.hpp>
#include <libtorrent/torrent_status.hpp>
#include <libtorrent/version.hpp>
#include <libtorrent/write_resume_data.hpp>

#include <boost/asio/ip/address.hpp>

#include <algorithm>
#include <cstdint>
#include <iostream>
#include <stdexcept>
#include <thread>
#include <utility>
#include <vector>

namespace odm {

using json = nlohmann::json;
using sp = lt::settings_pack;
using steady = std::chrono::steady_clock;

namespace {

// How often torrents are asked whether their resume data changed, and how
// long quitting waits for the last resume data.
constexpr auto kResumeCheck = std::chrono::seconds(30);
constexpr auto kQuitWait = std::chrono::seconds(10);

lt::span<char const> bytes(std::vector<char> const& v) {
  return {v.data(), static_cast<std::ptrdiff_t>(v.size())};
}

std::string state_name(lt::torrent_status::state_t s) {
  switch (s) {
    case lt::torrent_status::checking_files: return "checking";
    case lt::torrent_status::downloading_metadata: return "metadata";
    case lt::torrent_status::downloading: return "downloading";
    case lt::torrent_status::finished: return "finished";
    case lt::torrent_status::seeding: return "seeding";
    case lt::torrent_status::checking_resume_data: return "checking_resume_data";
    default: return "unknown";
  }
}

std::string socket_name(lt::socket_type_t t) {
  switch (t) {
    case lt::socket_type_t::tcp: return "tcp";
    case lt::socket_type_t::utp: return "utp";
    case lt::socket_type_t::tcp_ssl: return "tcp_ssl";
    case lt::socket_type_t::utp_ssl: return "utp_ssl";
    default: return "other";
  }
}

// ODM says "no limit" with 0; libtorrent with -1 for a torrent and 0 for the
// session.
int torrent_limit(json const& v) {
  int n = v.get<int>();
  return n > 0 ? n : -1;
}

int session_limit(json const& v) {
  int n = v.get<int>();
  return n > 0 ? n : 0;
}

lt::download_priority_t priority(json const& v) {
  int n = v.get<int>();
  if (n < 0 || n > 7) throw std::runtime_error("a file priority is 0 to 7");
  return lt::download_priority_t{static_cast<std::uint8_t>(n)};
}

std::vector<lt::download_priority_t> priorities(json const& list) {
  std::vector<lt::download_priority_t> out;
  for (auto const& v : list) out.push_back(priority(v));
  return out;
}

// file_index reads a file's index from a key of "names"; files is how many
// files the torrent has, -1 while that is unknown.
lt::file_index_t file_index(std::string const& s, int files) {
  std::size_t used = 0;
  int i = -1;
  try {
    i = std::stoi(s, &used);
  } catch (std::exception const&) {
  }
  if (used != s.size() || i < 0 || (files >= 0 && i >= files)) {
    throw std::runtime_error("no file " + s);
  }
  return lt::file_index_t{i};
}

lt::tcp::endpoint endpoint(std::string const& s) {
  auto colon = s.rfind(':');
  if (colon == std::string::npos) throw std::runtime_error("a peer is ip:port");
  std::string host = s.substr(0, colon);
  if (host.size() > 1 && host.front() == '[' && host.back() == ']') {
    host = host.substr(1, host.size() - 2);
  }
  int port = 0;
  try {
    port = std::stoi(s.substr(colon + 1));
  } catch (std::exception const&) {
  }
  if (port <= 0 || port > 65535) throw std::runtime_error("bad peer port in " + s);
  return lt::tcp::endpoint(boost::asio::ip::make_address(host), static_cast<unsigned short>(port));
}

void put_info_hashes(json& j, lt::info_hash_t const& ih) {
  if (ih.has_v1()) j["info_hash"] = hex(ih.v1.data(), ih.v1.size());
  if (ih.has_v2()) j["info_hash_v2"] = hex(ih.v2.data(), ih.v2.size());
}

// metadata describes a torrent's content, files in their original names.
// libtorrent 2.1 keeps trackers out of torrent_info: they are the caller's.
json metadata(lt::torrent_info const& ti, std::vector<std::string> const& trackers) {
  lt::file_storage const& fs = ti.layout();
  json files = json::array();
  for (lt::file_index_t i : fs.file_range()) {
    json f = {{"path", portable(fs.file_path(i))}, {"size", fs.file_size(i)}};
    if (fs.pad_file_at(i)) f["pad"] = true;
    files.push_back(std::move(f));
  }
  json j = {
      {"name", ti.name()},
      {"size", ti.total_size()},
      {"piece_size", ti.piece_length()},
      {"pieces", ti.num_pieces()},
      {"private", ti.priv()},
      {"files", files},
      {"trackers", json(trackers)},
  };
  put_info_hashes(j, ti.info_hashes());
  return j;
}

json status(std::string const& key, lt::torrent_status const& st) {
  json j = {
      {"key", key},
      {"state", state_name(st.state)},
      {"paused", static_cast<bool>(st.flags & lt::torrent_flags::paused)},
      {"has_metadata", st.has_metadata},
      {"size", st.total_wanted},
      {"done", st.total_wanted_done},
      {"total", st.total},
      {"total_done", st.total_done},
      {"download_rate", st.download_payload_rate},
      {"upload_rate", st.upload_payload_rate},
      {"downloaded", st.all_time_download},
      {"uploaded", st.all_time_upload},
      {"failed_bytes", st.total_failed_bytes},
      {"peers", st.num_peers},
      {"seeds", st.num_seeds},
      {"swarm_seeds", st.num_complete},
      {"swarm_peers", st.num_incomplete},
      {"finished", st.is_finished},
      {"seeding", st.is_seeding},
      {"moving", st.moving_storage},
      {"active_time", static_cast<std::int64_t>(st.active_duration.count())},
      {"seeding_time", static_cast<std::int64_t>(st.seeding_duration.count())},
  };
  if (st.errc) j["error"] = st.errc.message();
  return j;
}

}  // namespace

Helper::Helper(std::string state) : state_(std::move(state)) {
  lt::session_params params;
  if (!state_.empty()) {
    std::vector<char> buf;
    if (read_file(state_, buf)) {
      try {
        params = lt::read_session_params(bytes(buf), lt::session_handle::save_dht_state);
      } catch (std::exception const&) {
        params = lt::session_params();
      }
    }
  }
  sp s;
  s.set_int(sp::alert_mask, lt::alert_category::status | lt::alert_category::error |
                                lt::alert_category::storage | lt::alert_category::port_mapping);
  // Nothing listens, and no DHT, LSD or port mapping runs, until ODM's
  // settings say so.
  s.set_str(sp::listen_interfaces, "");
  s.set_bool(sp::enable_dht, false);
  s.set_bool(sp::enable_lsd, false);
  s.set_bool(sp::enable_upnp, false);
  s.set_bool(sp::enable_natpmp, false);
  s.set_bool(sp::ssrf_mitigation, true);
  // A paused torrent that resumes reconnects to its peers within seconds,
  // not the minute libtorrent waits by default before it calls a peer
  // again: pausing is common in a download manager.
  s.set_int(sp::min_reconnect_time, 10);
  // A magnet link held for ODM to choose its files wants nothing yet; that
  // must not drop the seeds it got the metadata from.
  s.set_bool(sp::close_redundant_connections, false);
  s.set_int(sp::stop_tracker_timeout, 2);
  s.set_int(sp::alert_queue_size, 10000);
  s.set_str(sp::user_agent, std::string("odm-libtorrent/") + ODM_LIBTORRENT_VERSION + " libtorrent/" +
                                LIBTORRENT_VERSION);
  params.settings = s;
  ses_ = std::make_unique<lt::session>(std::move(params));
}

int Helper::run() {
  ses_->set_alert_notify([this] {
    {
      std::lock_guard<std::mutex> lock(mu_);
      alerts_ = true;
    }
    wake_.notify_one();
  });
  emit({{"event", "hello"},
        {"protocol", kProtocol},
        {"version", ODM_LIBTORRENT_VERSION},
        {"libtorrent", LIBTORRENT_VERSION}});
  // The reader blocks on stdin; it ends with the process.
  std::thread([this] { read_input(); }).detach();

  auto now = steady::now();
  next_status_ = now + status_interval_;
  next_resume_check_ = now + kResumeCheck;
  while (!quitting_) {
    std::deque<std::string> lines;
    bool eof = false;
    {
      std::unique_lock<std::mutex> lock(mu_);
      auto until = std::min({next_status_, next_resume_check_, steady::now() + std::chrono::seconds(1)});
      wake_.wait_until(lock, until, [this] { return !lines_.empty() || eof_ || alerts_; });
      lines.swap(lines_);
      eof = eof_;
      alerts_ = false;
    }
    for (auto const& line : lines) {
      if (quitting_) break;
      handle_line(line);
    }
    handle_alerts();
    now = steady::now();
    if (status_interval_.count() > 0 && now >= next_status_) {
      ses_->post_torrent_updates({});
      next_status_ = now + status_interval_;
    }
    if (now >= next_resume_check_) {
      for (auto& [key, t] : torrents_) {
        if (!t.removing && t.handle.need_save_resume_data()) save_resume(t);
      }
      next_resume_check_ = now + kResumeCheck;
    }
    if (eof) quitting_ = true;
  }
  shutdown();
  return 0;
}

void Helper::read_input() {
  std::string line;
  while (std::getline(std::cin, line)) {
    if (!line.empty() && line.back() == '\r') line.pop_back();
    if (line.empty()) continue;
    {
      std::lock_guard<std::mutex> lock(mu_);
      lines_.push_back(std::move(line));
    }
    wake_.notify_one();
  }
  {
    std::lock_guard<std::mutex> lock(mu_);
    eof_ = true;
  }
  wake_.notify_one();
}

void Helper::handle_line(std::string const& line) {
  json reply = {{"id", nullptr}};
  try {
    json msg = json::parse(line);
    if (!msg.is_object()) throw std::runtime_error("a command is a JSON object");
    if (msg.contains("id")) reply["id"] = msg["id"];
    json out = dispatch(msg.value("cmd", std::string()), msg);
    reply["ok"] = true;
    for (auto& [k, v] : out.items()) reply[k] = v;
  } catch (std::exception const& e) {
    reply["ok"] = false;
    reply["error"] = e.what();
  }
  emit(reply);
}

Helper::json Helper::dispatch(std::string const& cmd, json const& msg) {
  if (cmd == "settings") return cmd_settings(msg);
  if (cmd == "inspect") return cmd_inspect(msg);
  if (cmd == "add") return cmd_add(msg);
  if (cmd == "remove") return cmd_remove(msg);
  if (cmd == "pause") {
    torrent(msg).handle.pause();
    return json::object();
  }
  if (cmd == "resume") {
    torrent(msg).handle.resume();
    return json::object();
  }
  if (cmd == "priorities") return cmd_priorities(msg);
  if (cmd == "rename") return cmd_rename(msg);
  if (cmd == "move") return cmd_move(msg);
  if (cmd == "limits") return cmd_limits(msg);
  if (cmd == "connect") return cmd_connect(msg);
  if (cmd == "status") return cmd_status(msg);
  if (cmd == "pieces") return cmd_pieces(msg);
  if (cmd == "file_progress") return cmd_file_progress(msg);
  if (cmd == "quit") {
    quitting_ = true;
    return json::object();
  }
  throw std::runtime_error("unknown command \"" + cmd + "\"");
}

Helper::json Helper::cmd_settings(json const& msg) {
  sp s;
  if (msg.contains("listen")) s.set_str(sp::listen_interfaces, msg["listen"].get<std::string>());
  if (msg.contains("incoming")) {
    bool on = msg["incoming"].get<bool>();
    s.set_bool(sp::enable_incoming_tcp, on);
    s.set_bool(sp::enable_incoming_utp, on);
  }
  if (msg.contains("port_forwarding")) {
    bool on = msg["port_forwarding"].get<bool>();
    s.set_bool(sp::enable_upnp, on);
    s.set_bool(sp::enable_natpmp, on);
  }
  if (msg.contains("dht")) s.set_bool(sp::enable_dht, msg["dht"].get<bool>());
  if (msg.contains("lsd")) s.set_bool(sp::enable_lsd, msg["lsd"].get<bool>());
  if (msg.contains("download_limit")) s.set_int(sp::download_rate_limit, session_limit(msg["download_limit"]));
  if (msg.contains("upload_limit")) s.set_int(sp::upload_rate_limit, session_limit(msg["upload_limit"]));
  if (msg.contains("user_agent")) s.set_str(sp::user_agent, msg["user_agent"].get<std::string>());
  if (msg.contains("peer_fingerprint")) {
    s.set_str(sp::peer_fingerprint, msg["peer_fingerprint"].get<std::string>());
  }
  if (msg.contains("proxy")) {
    json const& p = msg["proxy"];
    std::string type = p.value("type", std::string("none"));
    std::string user = p.value("user", std::string());
    int t = sp::none;
    if (type == "socks5") {
      t = user.empty() ? sp::socks5 : sp::socks5_pw;
    } else if (type == "http") {
      t = user.empty() ? sp::http : sp::http_pw;
    } else if (type != "none") {
      throw std::runtime_error("proxy type is none, socks5 or http");
    }
    s.set_int(sp::proxy_type, t);
    s.set_str(sp::proxy_hostname, p.value("host", std::string()));
    s.set_int(sp::proxy_port, p.value("port", 0));
    s.set_str(sp::proxy_username, user);
    s.set_str(sp::proxy_password, p.value("password", std::string()));
    s.set_bool(sp::proxy_peer_connections, p.value("peers", true));
    s.set_bool(sp::proxy_tracker_connections, true);
    s.set_bool(sp::proxy_hostnames, true);
  }
  if (msg.contains("status_interval")) {
    int ms = msg["status_interval"].get<int>();
    status_interval_ = std::chrono::milliseconds(ms > 0 ? ms : 0);
    next_status_ = ms > 0 ? steady::now() + status_interval_ : steady::time_point::max();
  }
  ses_->apply_settings(std::move(s));
  return json::object();
}

Helper::json Helper::cmd_inspect(json const& msg) {
  lt::add_torrent_params atp;
  if (msg.contains("torrent")) {
    atp = lt::load_torrent_file(native(msg["torrent"].get<std::string>()));
  } else if (msg.contains("magnet")) {
    atp = lt::parse_magnet_uri(msg["magnet"].get<std::string>());
  } else {
    throw std::runtime_error("inspect needs a torrent or a magnet link");
  }
  if (atp.ti) return metadata(*atp.ti, atp.trackers);
  json j = {{"name", atp.name}, {"trackers", json(atp.trackers)}};
  put_info_hashes(j, atp.info_hashes);
  return j;
}

Helper::json Helper::cmd_add(json const& msg) {
  std::string key = msg.at("key").get<std::string>();
  if (key.empty()) throw std::runtime_error("a torrent needs a key");
  if (torrents_.count(key) != 0) throw std::runtime_error("torrent \"" + key + "\" exists");
  std::string resume = msg.contains("resume") ? native(msg["resume"].get<std::string>()) : "";

  // Resume data first: it holds what is downloaded, and the metadata once
  // there was some. A file that cannot be read is started over.
  lt::add_torrent_params atp;
  bool resumed = false;
  std::vector<char> buf;
  if (!resume.empty() && read_file(resume, buf)) {
    lt::error_code ec;
    lt::add_torrent_params r = lt::read_resume_data(bytes(buf), ec);
    if (!ec) {
      atp = std::move(r);
      resumed = true;
    }
  }
  if (msg.contains("torrent")) {
    if (!resumed || !atp.ti) {
      lt::add_torrent_params t = lt::load_torrent_file(native(msg["torrent"].get<std::string>()));
      if (resumed) {
        atp.ti = t.ti;
      } else {
        atp = std::move(t);
      }
    }
  } else if (msg.contains("magnet")) {
    if (!resumed || !atp.ti) atp = lt::parse_magnet_uri(msg["magnet"].get<std::string>());
  } else if (!resumed) {
    throw std::runtime_error("add needs a torrent, a magnet link or resume data");
  }

  atp.save_path = native(msg.at("save_path").get<std::string>());
  int files = atp.ti ? atp.ti->num_files() : -1;
  if (msg.contains("names")) {
    if (!atp.ti) throw std::runtime_error("names need the torrent's metadata; rename once it is there");
    for (auto const& [k, v] : msg["names"].items()) {
      atp.renamed_files[file_index(k, files)] = native(v.get<std::string>());
    }
  }
  if (msg.contains("priorities")) atp.file_priorities = priorities(msg["priorities"]);
  // ODM runs its own queue: libtorrent's automatic management stays off.
  atp.flags &= ~lt::torrent_flags::auto_managed;
  atp.flags &= ~lt::torrent_flags::paused;
  atp.flags |= lt::torrent_flags::duplicate_is_error;
  if (msg.value("paused", false)) atp.flags |= lt::torrent_flags::paused;
  if (msg.value("hold", false)) atp.flags |= lt::torrent_flags::default_dont_download;
  if (msg.contains("download_limit")) atp.download_limit = torrent_limit(msg["download_limit"]);
  if (msg.contains("upload_limit")) atp.upload_limit = torrent_limit(msg["upload_limit"]);
  if (msg.contains("peers")) {
    for (auto const& p : msg["peers"]) atp.peers.push_back(endpoint(p.get<std::string>()));
  }

  lt::torrent_handle h = ses_->add_torrent(std::move(atp));
  Torrent t;
  t.key = key;
  t.handle = h;
  t.info_hashes = h.info_hashes();
  t.resume = resume;
  json out = {{"resumed", resumed}};
  put_info_hashes(out, t.info_hashes);
  torrents_.emplace(key, std::move(t));
  // The metadata event, for a torrent that has it already, follows from
  // add_torrent_alert, after this reply.
  return out;
}

Helper::json Helper::cmd_remove(json const& msg) {
  Torrent& t = torrent(msg);
  t.removing = true;
  t.delete_files = msg.value("delete_files", false);
  ses_->remove_torrent(t.handle, t.delete_files ? lt::session_handle::delete_files : lt::remove_flags_t{});
  if (!t.resume.empty()) remove_file(t.resume);
  return json::object();
}

Helper::json Helper::cmd_priorities(json const& msg) {
  Torrent& t = torrent(msg);
  t.handle.prioritize_files(priorities(msg.at("priorities")));
  return json::object();
}

Helper::json Helper::cmd_rename(json const& msg) {
  Torrent& t = torrent(msg);
  auto ti = t.handle.torrent_file();
  if (!ti) throw std::runtime_error("the torrent has no metadata yet");
  int files = ti->num_files();
  std::vector<std::pair<lt::file_index_t, std::string>> names;
  for (auto const& [k, v] : msg.at("names").items()) {
    names.emplace_back(file_index(k, files), native(v.get<std::string>()));
  }
  for (auto const& [i, name] : names) t.handle.rename_file(i, name);
  return json::object();
}

Helper::json Helper::cmd_move(json const& msg) {
  Torrent& t = torrent(msg);
  t.handle.move_storage(native(msg.at("save_path").get<std::string>()), lt::move_flags_t::fail_if_exist);
  return json::object();
}

Helper::json Helper::cmd_limits(json const& msg) {
  Torrent& t = torrent(msg);
  if (msg.contains("download_limit")) t.handle.set_download_limit(torrent_limit(msg["download_limit"]));
  if (msg.contains("upload_limit")) t.handle.set_upload_limit(torrent_limit(msg["upload_limit"]));
  return json::object();
}

Helper::json Helper::cmd_connect(json const& msg) {
  Torrent& t = torrent(msg);
  t.handle.connect_peer(endpoint(msg.at("peer").get<std::string>()));
  return json::object();
}

Helper::json Helper::cmd_status(json const& msg) {
  json list = json::array();
  if (msg.contains("key")) {
    Torrent& t = torrent(msg);
    list.push_back(status(t.key, t.handle.status({})));
  } else {
    for (auto& [key, t] : torrents_) {
      if (!t.removing) list.push_back(status(key, t.handle.status({})));
    }
  }
  return {{"torrents", list}};
}

Helper::json Helper::cmd_pieces(json const& msg) {
  Torrent& t = torrent(msg);
  lt::torrent_status st = t.handle.status(lt::torrent_handle::query_pieces);
  int n = st.pieces.size();
  // One bit a piece, the first piece in the high bit, as BitTorrent's own
  // bitfield message has it.
  std::vector<unsigned char> bits(static_cast<std::size_t>((n + 7) / 8), 0);
  for (int i = 0; i < n; ++i) {
    if (st.pieces.get_bit(lt::piece_index_t{i})) bits[i / 8] |= static_cast<unsigned char>(0x80 >> (i % 8));
  }
  return {{"count", n}, {"have", base64(bits)}};
}

Helper::json Helper::cmd_file_progress(json const& msg) {
  Torrent& t = torrent(msg);
  return {{"files", t.handle.file_progress()}};
}

void Helper::handle_alerts() {
  std::vector<lt::alert*> alerts;
  ses_->pop_alerts(&alerts);
  for (lt::alert* a : alerts) {
    try {
      on_alert(a);
    } catch (std::exception const& e) {
      std::cerr << "odm-libtorrent: " << a->what() << ": " << e.what() << '\n';
    }
  }
}

void Helper::on_alert(lt::alert* a) {
  if (auto* p = lt::alert_cast<lt::state_update_alert>(a)) {
    json list = json::array();
    for (auto const& st : p->status) {
      Torrent* t = find(st.handle);
      if (t != nullptr && !t->removing) list.push_back(status(t->key, st));
    }
    if (!list.empty()) emit({{"event", "status"}, {"torrents", list}});
    return;
  }
  if (auto* p = lt::alert_cast<lt::add_torrent_alert>(a)) {
    if (p->error) return;
    Torrent* t = find(p->handle);
    if (t != nullptr && p->handle.torrent_file()) on_metadata(*t);
    return;
  }
  if (auto* p = lt::alert_cast<lt::metadata_received_alert>(a)) {
    if (Torrent* t = find(p->handle)) on_metadata(*t);
    return;
  }
  if (auto* p = lt::alert_cast<lt::metadata_failed_alert>(a)) {
    if (Torrent* t = find(p->handle)) {
      emit({{"event", "error"}, {"key", t->key}, {"error", "bad metadata: " + p->error.message()}});
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::torrent_finished_alert>(a)) {
    Torrent* t = find(p->handle);
    if (t == nullptr || t->removing) return;
    // A torrent with nothing wanted yet (held for ODM to choose its files)
    // is "finished" as well; that is not news. Otherwise ODM hears of it
    // once libtorrent's write cache is on the disk, so that the files it
    // then opens, renames or checks are whole.
    if (t->handle.status({}).total_wanted > 0) {
      t->finishing = true;
      t->handle.flush_cache();
    }
    save_resume(*t);
    return;
  }
  if (auto* p = lt::alert_cast<lt::cache_flushed_alert>(a)) {
    if (flushing_ > 0) --flushing_;
    Torrent* t = find(p->handle);
    if (t != nullptr && t->finishing && !t->removing) {
      t->finishing = false;
      emit({{"event", "finished"}, {"key", t->key}});
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::torrent_paused_alert>(a)) {
    Torrent* t = find(p->handle);
    if (t != nullptr && !t->removing) save_resume(*t);
    return;
  }
  if (auto* p = lt::alert_cast<lt::torrent_error_alert>(a)) {
    if (Torrent* t = find(p->handle)) {
      emit({{"event", "error"}, {"key", t->key}, {"error", p->error.message()}, {"file", portable(p->filename())}});
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::file_error_alert>(a)) {
    if (Torrent* t = find(p->handle)) {
      emit({{"event", "error"},
            {"key", t->key},
            {"error", p->error.message()},
            {"file", portable(p->filename())},
            {"op", lt::operation_name(p->op)}});
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::file_renamed_alert>(a)) {
    if (Torrent* t = find(p->handle)) {
      emit({{"event", "renamed"},
            {"key", t->key},
            {"index", static_cast<int>(p->index)},
            {"path", portable(p->new_name())}});
      save_resume(*t);
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::file_rename_failed_alert>(a)) {
    if (Torrent* t = find(p->handle)) {
      emit({{"event", "rename_failed"},
            {"key", t->key},
            {"index", static_cast<int>(p->index)},
            {"error", p->error.message()}});
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::storage_moved_alert>(a)) {
    if (Torrent* t = find(p->handle)) {
      emit({{"event", "moved"}, {"key", t->key}, {"path", portable(p->storage_path())}});
      save_resume(*t);
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::storage_moved_failed_alert>(a)) {
    if (Torrent* t = find(p->handle)) {
      emit({{"event", "move_failed"},
            {"key", t->key},
            {"error", p->error.message()},
            {"file", portable(p->file_path())}});
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::save_resume_data_alert>(a)) {
    if (outstanding_ > 0) --outstanding_;
    Torrent* t = find(p->handle);
    if (t == nullptr) return;
    t->saving = false;
    if (t->removing || t->resume.empty()) return;
    std::string err;
    if (write_file(t->resume, lt::write_resume_data_buf(p->params), err)) {
      emit({{"event", "resume_saved"}, {"key", t->key}});
    } else {
      emit({{"event", "error"}, {"key", t->key}, {"error", err}});
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::save_resume_data_failed_alert>(a)) {
    if (outstanding_ > 0) --outstanding_;
    if (Torrent* t = find(p->handle)) t->saving = false;
    return;
  }
  if (auto* p = lt::alert_cast<lt::torrent_removed_alert>(a)) {
    Torrent* t = find_removing(p->info_hashes);
    if (t == nullptr) return;
    // Files to delete: removed once they are gone.
    if (t->delete_files && t->has_metadata) return;
    emit({{"event", "removed"}, {"key", t->key}});
    forget(t->key);
    return;
  }
  if (auto* p = lt::alert_cast<lt::torrent_deleted_alert>(a)) {
    if (Torrent* t = find_removing(p->info_hashes)) {
      emit({{"event", "removed"}, {"key", t->key}});
      forget(t->key);
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::torrent_delete_failed_alert>(a)) {
    if (Torrent* t = find_removing(p->info_hashes)) {
      emit({{"event", "removed"}, {"key", t->key}, {"error", p->error.message()}});
      forget(t->key);
    }
    return;
  }
  if (auto* p = lt::alert_cast<lt::listen_succeeded_alert>(a)) {
    emit({{"event", "listening"},
          {"address", p->address.to_string()},
          {"port", p->port},
          {"type", socket_name(p->socket_type)}});
    return;
  }
  if (auto* p = lt::alert_cast<lt::listen_failed_alert>(a)) {
    emit({{"event", "listen_failed"},
          {"address", p->listen_interface()},
          {"port", p->port},
          {"type", socket_name(p->socket_type)},
          {"error", p->error.message()}});
    return;
  }
  if (auto* p = lt::alert_cast<lt::portmap_alert>(a)) {
    emit({{"event", "portmap"},
          {"port", p->external_port},
          {"via", p->map_transport == lt::portmap_transport::upnp ? "upnp" : "natpmp"}});
    return;
  }
  if (auto* p = lt::alert_cast<lt::portmap_error_alert>(a)) {
    emit({{"event", "portmap_failed"},
          {"via", p->map_transport == lt::portmap_transport::upnp ? "upnp" : "natpmp"},
          {"error", p->error.message()}});
    return;
  }
}

void Helper::on_metadata(Torrent& t) {
  auto ti = t.handle.torrent_file();
  if (!ti) return;
  t.has_metadata = true;
  std::vector<std::string> trackers;
  for (auto const& e : t.handle.trackers()) trackers.push_back(e.url);
  json j = metadata(*ti, trackers);
  j["event"] = "metadata";
  j["key"] = t.key;
  lt::add_torrent_params p;
  p.ti = std::const_pointer_cast<lt::torrent_info>(ti);
  p.trackers = trackers;
  j["magnet"] = lt::make_magnet_uri(p);
  emit(j);
  // Keep the metadata, so that the torrent need not fetch it again.
  save_resume(t);
}

void Helper::save_resume(Torrent& t) {
  if (t.saving || t.removing || t.resume.empty()) return;
  t.saving = true;
  ++outstanding_;
  t.handle.save_resume_data(lt::torrent_handle::save_info_dict);
}

void Helper::shutdown() {
  // Stop the transfers and write out the cache, so that the last resume
  // data counts every piece checked.
  ses_->pause();
  for (auto& [key, t] : torrents_) {
    if (t.removing) continue;
    t.handle.flush_cache();
    ++flushing_;
  }
  auto until = steady::now() + kQuitWait;
  while (flushing_ > 0 && steady::now() < until) {
    ses_->wait_for_alert(std::chrono::milliseconds(100));
    handle_alerts();
  }
  for (auto& [key, t] : torrents_) {
    if (!t.removing) save_resume(t);
  }
  while (outstanding_ > 0 && steady::now() < until) {
    ses_->wait_for_alert(std::chrono::milliseconds(100));
    handle_alerts();
  }
  if (!state_.empty()) {
    std::string err;
    lt::session_params params = ses_->session_state(lt::session_handle::save_dht_state);
    if (!write_file(state_, lt::write_session_params_buf(params, lt::session_handle::save_dht_state), err)) {
      std::cerr << "odm-libtorrent: " << err << '\n';
    }
  }
  emit({{"event", "bye"}});
  // Trackers are told the torrents stopped, for at most stop_tracker_timeout.
  lt::session_proxy proxy = ses_->abort();
  ses_.reset();
}

Torrent& Helper::torrent(json const& msg) {
  std::string key = msg.at("key").get<std::string>();
  auto it = torrents_.find(key);
  if (it == torrents_.end() || it->second.removing) throw std::runtime_error("no torrent \"" + key + "\"");
  return it->second;
}

Torrent* Helper::find(lt::torrent_handle const& h) {
  for (auto& [key, t] : torrents_) {
    if (t.handle == h) return &t;
  }
  return nullptr;
}

Torrent* Helper::find_removing(lt::info_hash_t const& ih) {
  for (auto& [key, t] : torrents_) {
    if (t.removing && t.info_hashes == ih) return &t;
  }
  return nullptr;
}

void Helper::forget(std::string const& key) {
  std::string k = key;  // key may live in the torrent being erased
  torrents_.erase(k);
}

void Helper::emit(json const& j) {
  std::cout << j.dump(-1, ' ', false, json::error_handler_t::replace) << '\n' << std::flush;
  // ODM is gone.
  if (!std::cout) quitting_ = true;
}

}  // namespace odm
