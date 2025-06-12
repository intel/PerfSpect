/*
 * util.hpp
 */

#ifndef UTIL_HPP_
#define UTIL_HPP_

#include <utility>
#include <iterator>
#include <algorithm>
#include <cassert>

/*
 * Split a string delimited by sep.
 *
 * See https://stackoverflow.com/a/7408245/149138
 */
static inline std::vector<std::string> split(const std::string &text, const std::string &sep) {
  std::vector<std::string> tokens;
  std::size_t start = 0, end = 0;
  while ((end = text.find(sep, start)) != std::string::npos) {
    tokens.push_back(text.substr(start, end - start));
    start = end + sep.length();
  }
  tokens.push_back(text.substr(start));
  return tokens;
}

template <typename C>
static inline std::string join(const C& c, std::string sep) {
    std::string ret;
    for (auto& e : c) {
        if (!ret.empty()) {
            ret += sep;
        }
        ret += std::to_string(e);
    }
    return ret;
}

/**
 * Like std::transform, but allocates and returns a std::vector for the result.
 */
template <typename Itr, typename F>
auto transformr(Itr begin, Itr end, F f) -> std::vector<decltype(f(std::declval<typename std::iterator_traits<Itr>::value_type>()))> {
    decltype(transformr(begin, end, f)) ret;
    ret.reserve(std::distance(begin, end));
    std::transform(begin, end, std::back_inserter(ret), f);
    return ret;
}

template <typename C, typename F>
auto transformv(const C& c, F f) -> std::vector<decltype(f(*c.begin()))> {
    return transformr(std::begin(c), std::end(c), f);
}

template <typename Itr>
static inline auto concurrency(Itr start, Itr stop) -> typename std::iterator_traits<Itr>::value_type {
    if (start == stop) {
        return {0, 0}; // early out for empty range simplifies some logic below
    }

    using T1 = decltype(concurrency(start, stop).first);

    struct event {
        T1 stamp;
        enum Type { START, STOP } type;
        event(T1 stamp, Type type) : stamp{stamp}, type{type} {}
    };

    std::vector<event> events;
    events.reserve(std::distance(start, stop));
    T1 sum_top{}, sum_bottom{};
    for (Itr i = start; i != stop; i++) {
        sum_top += i->second - i->first;
        events.emplace_back(i->first, event::START);
        events.emplace_back(i->second,event::STOP);
    }

    std::sort(events.begin(), events.end(), [](event l, event r){ return l.stamp < r.stamp; });

    size_t count = 0;
    const event* last_event = nullptr;
    for (auto& event : events) {
        assert(count > 0 || event.type == event::START);
        if (count != 0) {
            assert(last_event);
            T1 period = event.stamp - last_event->stamp;
            // active interval, accumulate the numerator and denominators
            sum_bottom += period;
        }
        last_event = &event;
        count += event.type == event::START ? 1 : -1;
    }

    assert(count == 0);

    return {sum_top, sum_bottom};
}

/**
 * Nested concurrency.
 *
 * Returns a pair, where second is the sum of all of the inner intervals, and first is the weighted sum of
 * all of the inner interval, weighted by the number of concurrent outer intervals. That is, if there are
 * two concurrent outer intervals for the entire period of an inner interval, the value contributed to first
 * is twice the size of its interval.
 *
 * This calculates a concurrency value somewhat like concurrency(), except that the evaluated intervals are
 * broken into two sets: inner and outer (although these names are somewhat arbitrary). The returned value
 * is the concurrency of the inner ranges evaluated against the outer ranges. That is, the concurrency value
 * at any point for any nested range is not related to any other concurrent nested ranges, but the number of
 * concurrent outer ranges.
 *
 * This intuition is that this is a useful for figure for evaluating concurrent benchmarks where each benchmark
 * thread has a nested structure like:
 *
 * {
 *   // OUTER region
 *   {
 *     // INNER region
 *   }
 * }
 *
 * That is, the OUTER region encloses the INNER. In this scenio, the INNER region may be the timed one, while the OUTER
 * region is performing the same type of operations as the INNER, but not timed. In particular, the effect on other
 * threads is similar in the OUTER and INNER regions. One may way to evaluate whether all INNER regions were executed
 * during a time when the OUTER region on all other threads was active.
 */
template <typename Itr>
static inline auto nested_concurrency(Itr starto, Itr stopo, Itr starti, Itr stopi) -> typename std::iterator_traits<Itr>::value_type {
    if (starti == stopi) {
        return {0, 0}; // early out for empty range simplifies some logic below
    }

    using T1 = decltype(nested_concurrency(stopo, stopo, stopo, stopo).first);

    struct event {
        T1 stamp;
        enum Type { STARTO, STOPO, STARTI, STOPI } type;
        event(T1 stamp, Type type) : stamp{stamp}, type{type} {}
    };

    std::vector<event> events;
    events.reserve(std::distance(starto, stopo) + std::distance(starti, stopi));
    for (Itr i = starto; i != stopo; i++) {
        events.emplace_back(i->first, event::STARTO);
        events.emplace_back(i->second,event::STOPO);
    }
    T1 sum_top{}, sum_bottom{};
    for (Itr i = starti; i != stopi; i++) {
        sum_bottom += i->second - i->first;
        events.emplace_back(i->first, event::STARTI);
        events.emplace_back(i->second,event::STOPI);
    }

    std::sort(events.begin(), events.end(), [](event l, event r){ return l.stamp < r.stamp; });

    /*     00011242110  == 12 (out of a possible 11 * 2 == 22)
     *        IIIII
     *          IIIIII
     *       CCCCC
     *           CCC
     */
    size_t ocount = 0, icount = 0;
    T1 last_stamp = events.front().stamp;
    for (auto& event : events) {
        sum_top += ocount * icount * (event.stamp - last_stamp);
        switch (event.type) {
        case event::STARTO:
            ocount++;
            break;
        case event::STOPO:
            assert(ocount > 0);
            ocount--;
            break;
        case event::STARTI:
            icount++;
            break;
        case event::STOPI:
            assert(icount > 0);
            icount--;
            break;
        }
        last_stamp = event.stamp;
    }

    assert(ocount == 0);
    assert(icount == 0);

    return {sum_top, sum_bottom};
}

/**
 * Linearly remap value from the input range to the output range. That is, return the value that represents in outrange the
 * relative position of the input value in outrange.
 */
static inline double remap(double value, double inrange_start, double inrange_end, double outrange_start, double outrange_end) {
    return outrange_start + (outrange_end - outrange_start) / (inrange_end - inrange_start) * (value - inrange_start);
}

/**
 * The concurrency ratio for the pairs in the range [start, stop).
 *
 * Intuitively, a ratio of 1.0 means maximum overlap, while a ratio of 0.0 means all
 * the ranges were distinct.
 *
 * Simply a shortcut for (c.first - c.second) / (c.second * std::distance(start, stop)) where
 * c = concurrency(start, stop).
 */
template <typename Itr>
static inline double conc_ratio(Itr start, Itr stop) {
    size_t N = std::distance(start, stop);
    if (N == 1) {
        return 1.0; // special "by definition" case since remap doesn't work in this case
    }
    auto conc = concurrency(start, stop);
    // gives a ratio between N and 1 where N is the number of ranges
    double raw_ratio = conc.first/((double)conc.second);
    return remap(raw_ratio, 1.0, N, 0.0, 1.0);
}

template <typename Itr>
static inline double nconc_ratio(Itr starto, Itr stopo, Itr starti, Itr stopi) {
    size_t ocount = std::distance(starto, stopo);
    if (ocount == 0) {
        return 0.0;
    }
    auto conc = nested_concurrency(starto, stopo, starti, stopi);
    // gives a ratio between N and 1 where N is the number of ranges
    double raw_ratio = conc.first/((double)conc.second);
    if (ocount == 1) {
        return raw_ratio;
    }
    return remap(raw_ratio, 1, ocount, 0.0, 1.0);
}


#endif /* UTIL_HPP_ */
