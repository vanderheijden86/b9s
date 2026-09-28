#!/bin/sh
# Rebuilds examples/space-programme/.beads/issues.jsonl from nothing, with the
# bd on PATH, in an embedded project under a temporary directory. The issues
# are synthetic: Red Harbor, a made-up crewed Moon-and-Mars programme with
# epics, features, tasks, bugs, chores, blocked work and releases.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# Blank every Beads and Dolt setting of the calling shell, so no server
# configuration (a direnv .envrc, a shell profile) can point these writes at a
# real database. awk rather than sed: BSD sed has no alternation in a basic
# regular expression, and a pattern that silently matches nothing unsets
# nothing.
for name in $(env | awk -F= '/^(BEADS|BD|DOLT)_/ { print $1 }'); do
	unset "$name"
done
export BEADS_DIR="$work/.beads" BEADS_DOLT_AUTO_START=0 BD_NON_INTERACTIVE=1 BEADS_ACTOR=mira
# Due and defer dates are calendar days; read them in UTC so the file does not
# depend on the timezone of whoever regenerates it.
export TZ=UTC

# bd records the git identity as each issue's owner. Give it a made-up one
# rather than the identity of whoever runs this script.
printf '[user]\n\tname = mira\n\temail = mira@example.com\n' >"$work/gitconfig"
export GIT_CONFIG_GLOBAL="$work/gitconfig" GIT_CONFIG_NOSYSTEM=1

cd "$work"
bd init --prefix mars --quiet --skip-hooks >/dev/null
# Refuse to write anything unless the project is embedded, not a server.
grep -q '"dolt_mode": "embedded"' .beads/metadata.json

# bd prints a hint about beads.role on stderr for every command.
new() { bd create --silent "$@" 2>/dev/null; }
# bd closes an assigned issue only for its assignee.
close_as() { who=$1; shift; BEADS_ACTOR=$who bd close "$@" >/dev/null 2>&1; }
say() { who=$1; id=$2; shift 2; BEADS_ACTOR=$who bd comments add "$id" -- "$*" >/dev/null 2>&1; }
dep() { bd dep add "$1" "$2" >/dev/null 2>&1; }

# ---------------------------------------------------------------- epics

flight=$(new --type=epic --priority=0 --title="Integrated flight test 1" --assignee=mira --labels=propulsion,safety \
	--description="Stack booster and ship on the pad and fly the first full profile over the sea. Reference: https://en.wikipedia.org/wiki/Reusable_launch_vehicle")
launcher=$(new --type=epic --priority=0 --title="Reusable heavy launcher with booster catch" --assignee=tomas --labels=propulsion \
	--description="Fly the booster back to the launch tower and catch it with the tower arms, so it can refly within days. Background on reusable launchers: https://en.wikipedia.org/wiki/Reusable_launch_vehicle")
transfer=$(new --type=epic --priority=1 --title="Ship-to-ship propellant transfer in orbit" --assignee=ines --labels=propulsion \
	--description="Move cryogenic propellant from tanker ships into a depot ship in low Earth orbit, enough to send a lander to the Moon. Background: https://en.wikipedia.org/wiki/Propellant_depot and https://www.nasa.gov/humans-in-space/human-landing-system/")
landing=$(new --type=epic --priority=1 --title="Uncrewed then crewed lunar landing" --assignee=kofi --labels=avionics,safety \
	--description="Land the lander near the lunar south pole without crew first, then with two crew members. Public reference plans: https://www.nasa.gov/humans-in-space/artemis/ and https://www.nasa.gov/mission/artemis-iii/")
habitat=$(new --type=epic --priority=2 --title="Life support and surface habitat" --assignee=yuki --labels=life-support \
	--description="Keep four crew alive for 30 days on the surface: air, water, thermal control and a pressurised habitat. Reference: https://www.nasa.gov/reference/environmental-control-and-life-support-systems-eclss/ and https://www.nasa.gov/moontomarsarchitecture/")
mars=$(new --type=epic --priority=2 --title="Mars cargo flight in a launch window" --assignee=lena --labels=propulsion,avionics \
	--description="Send an uncrewed cargo ship to Mars in the next transfer window and land it intact. Background: https://www.nasa.gov/humans-in-space/humans-to-mars/ and https://en.wikipedia.org/wiki/Launch_window")
ground=$(new --type=epic --priority=1 --title="Ground and launch operations" --assignee=dario --labels=ground-ops \
	--description="Pad, tank farm, launch tower and range work needed to launch every few days. Reference: https://www.nasa.gov/humans-in-space/exploration-ground-systems/")

# ------------------------------------------------ integrated flight test 1 (all done)

ft_stack=$(new --type=task --priority=0 --parent="$flight" --assignee=dario --title="Stack ship on booster with the tower arms" --labels=ground-ops)
ft_static=$(new --type=task --priority=0 --parent="$flight" --assignee=tomas --title="Full-duration static fire of all booster engines" --labels=propulsion,safety)
ft_fts=$(new --type=task --priority=0 --parent="$flight" --assignee=kofi --title="Certify the flight termination system" --labels=avionics,safety \
	--acceptance="Range safety signs off on both receivers and both charges.")

# ------------------------------------------------ launcher and booster catch

catch=$(new --type=feature --priority=0 --parent="$launcher" --assignee=tomas --title="Booster catch at the launch tower" --labels=propulsion,ground-ops \
	--design="The booster hovers beside the tower; two arms close under the catch pins. No landing legs.")
arm_seq=$(new --type=task --priority=0 --parent="$catch" --assignee=dario --title="Qualify the chopstick arm catch sequence" --labels=ground-ops,safety \
	--acceptance="Twenty dry runs with a mass simulator; arm close time under 1.2 s every time.")
boostback=$(new --type=task --priority=1 --parent="$catch" --assignee=tomas --title="Boostback burn lands within 5 m of the tower" --labels=propulsion,avionics)
divert=$(new --type=task --priority=1 --parent="$catch" --assignee=kofi --title="Abort divert to the sea if the tower is not go" --labels=avionics,safety)
catch_bug=$(new --type=bug --priority=0 --parent="$catch" --assignee=tomas --title="Catch pin load cell reads zero after hot-stage separation" --labels=avionics \
	--description="Both load cells on the port pin drop to zero for 0.4 s after separation. The arm controller treats that as no booster present.")
engines=$(new --type=feature --priority=1 --parent="$launcher" --assignee=tomas --title="Engine relight and throttle for landing" --labels=propulsion)
relight=$(new --type=task --priority=1 --parent="$engines" --assignee=tomas --title="Three-engine relight in under 2 s" --labels=propulsion)
turnaround=$(new --type=task --priority=2 --parent="$launcher" --assignee=dario --title="Refly the booster within 72 hours" --labels=ground-ops)
heat_bug=$(new --type=bug --priority=1 --parent="$launcher" --assignee=tomas --title="Engine bay heat shield tiles crack on reentry" --labels=propulsion,safety)

# ------------------------------------------------ propellant transfer

depot=$(new --type=feature --priority=1 --parent="$transfer" --assignee=ines --title="Depot ship holds propellant for 30 days" --labels=propulsion)
boiloff=$(new --type=task --priority=1 --parent="$depot" --assignee=ines --title="Boil-off below 0.5%/day in the depot tank" --labels=propulsion \
	--acceptance="Measured over 14 days in a thermal vacuum chamber at full sun.")
mli=$(new --type=task --priority=2 --parent="$depot" --assignee=ines --title="Multi-layer insulation blankets on the depot dome" --labels=propulsion)
docking=$(new --type=feature --priority=1 --parent="$transfer" --assignee=kofi --title="Automated tanker docking" --labels=avionics)
lidar=$(new --type=task --priority=1 --parent="$docking" --assignee=kofi --title="Relative navigation with lidar from 2 km to contact" --labels=avionics)
latch=$(new --type=task --priority=2 --parent="$docking" --assignee=ines --title="Quick-disconnect propellant couplers mate under 0.5 bar" --labels=propulsion)
settle=$(new --type=task --priority=1 --parent="$transfer" --assignee=ines --title="Settling thrust keeps liquid over the tank outlet" --labels=propulsion \
	--description="In free fall the liquid floats. A small constant thrust settles it over the outlet before and during transfer.")
flow=$(new --type=task --priority=1 --parent="$transfer" --assignee=ines --title="Transfer 100 t of liquid oxygen between two ships" --labels=propulsion)
sensor_bug=$(new --type=bug --priority=1 --parent="$transfer" --assignee=kofi --title="Tank level sensor drifts 3% in microgravity" --labels=avionics)

# ------------------------------------------------ lunar landing

uncrewed=$(new --type=feature --priority=1 --parent="$landing" --assignee=kofi --title="Uncrewed demonstration landing" --labels=avionics)
hazard=$(new --type=task --priority=1 --parent="$uncrewed" --assignee=kofi --title="Terrain-relative navigation picks a flat 20 m site" --labels=avionics)
plume=$(new --type=task --priority=2 --parent="$uncrewed" --assignee=lena --title="Keep the engine plume from digging a crater" --labels=propulsion,safety \
	--design="Mid-body landing thrusters fire for the final 30 m, so the main engines never face the regolith.")
legs=$(new --type=task --priority=2 --parent="$uncrewed" --assignee=lena --title="Landing legs level the ship on a 10 degree slope" --labels=propulsion)
crewed=$(new --type=feature --priority=1 --parent="$landing" --assignee=kofi --title="Crewed landing and return" --labels=avionics,safety)
elevator=$(new --type=task --priority=2 --parent="$crewed" --assignee=yuki --title="Crew elevator from the cabin to the surface" --labels=life-support,safety)
abort=$(new --type=task --priority=0 --parent="$crewed" --assignee=kofi --title="Abort to orbit from any point in the descent" --labels=avionics,safety)

# ------------------------------------------------ life support and habitat

eclss=$(new --type=feature --priority=1 --parent="$habitat" --assignee=yuki --title="Closed-loop air and water" --labels=life-support)
co2=$(new --type=task --priority=1 --parent="$eclss" --assignee=yuki --title="CO2 scrubber holds 0.3% for four crew" --labels=life-support)
fire=$(new --type=task --priority=1 --parent="$eclss" --assignee=yuki --title="Fire detection and suppression in the cabin" --labels=life-support,safety)
hab=$(new --type=feature --priority=2 --parent="$habitat" --assignee=noor --title="Surface habitat module" --labels=life-support)
airlock=$(new --type=task --priority=2 --parent="$hab" --assignee=noor --title="Dust-tolerant airlock seals" --labels=life-support)
radiation=$(new --type=task --priority=2 --parent="$hab" --assignee=noor --title="Storm shelter keeps dose under the 30-day limit" --labels=life-support,safety)
power=$(new --type=task --priority=3 --parent="$hab" --title="Power through the 14-day lunar night" --labels=life-support)
odor_bug=$(new --type=bug --priority=3 --parent="$habitat" --assignee=yuki --title="Charcoal filter saturates after 9 days, not 30" --labels=life-support)

# ------------------------------------------------ Mars cargo flight

trajectory=$(new --type=task --priority=2 --parent="$mars" --assignee=lena --title="Choose the transfer trajectory for the next window" --labels=avionics \
	--description="A near-Hohmann transfer: https://en.wikipedia.org/wiki/Hohmann_transfer_orbit")
edl=$(new --type=feature --priority=2 --parent="$mars" --assignee=lena --title="Entry, descent and landing on Mars" --labels=propulsion,avionics)
aerocapture=$(new --type=task --priority=2 --parent="$edl" --assignee=lena --title="Heat shield survives 7.5 km/s entry" --labels=propulsion,safety)
flip=$(new --type=task --priority=2 --parent="$edl" --title="Flip-and-burn at 500 m in thin atmosphere" --labels=propulsion,avionics)
cruise=$(new --type=task --priority=3 --parent="$mars" --title="Six-month cruise with the tanks cold" --labels=propulsion)
payload=$(new --type=task --priority=4 --parent="$mars" --title="Pack the cargo manifest: rovers, solar arrays, spare parts" --labels=ground-ops)

# ------------------------------------------------ ground operations

tankfarm=$(new --type=feature --priority=1 --parent="$ground" --assignee=dario --title="Tank farm fills a full stack in 45 minutes" --labels=ground-ops)
subcool=$(new --type=task --priority=1 --parent="$tankfarm" --assignee=dario --title="Subcool the liquid methane before loading" --labels=ground-ops,propulsion)
quench=$(new --type=task --priority=2 --parent="$ground" --assignee=dario --title="Water deluge plate survives ten launches" --labels=ground-ops,safety)
range=$(new --type=task --priority=2 --parent="$ground" --title="Clear the sea range within 4 hours of the window" --labels=ground-ops,safety)
valve_bug=$(new --type=bug --priority=1 --parent="$ground" --assignee=dario --title="Oxygen fill valve sticks below -180 C" --labels=ground-ops)
weather=$(new --type=chore --priority=3 --parent="$ground" --assignee=dario --title="Replace the upper-level wind balloon station" --labels=ground-ops)
lint=$(new --type=chore --priority=4 --assignee=kofi --title="Move flight software to the new compiler release" --labels=avionics)
docs=$(new --type=chore --priority=4 --title="Rewrite the pad safety briefing" --labels=safety)

# ------------------------------------------------ releases (milestones)

ft1=$(new --type=milestone --priority=0 --due=2026-04-15 --assignee=mira --title="Flight Test 1" \
	--description="First full profile of the integrated stack. Released when the flight test epic closes.")
lunar_demo=$(new --type=milestone --priority=1 --due=2027-06-30 --assignee=mira --title="Lunar Demo" \
	--description="Catch the booster, fill the depot in orbit and land on the Moon without crew.")
crewed_landing=$(new --type=milestone --priority=1 --due=2028-09-30 --assignee=mira --title="Crewed Landing" \
	--description="Two crew land, live on the surface and return.")

# A release depends on each epic it ships (ADR 0026). The Mars cargo epic has
# no release yet, so the release views list it as unscheduled.
dep "$ft1" "$flight"
dep "$lunar_demo" "$launcher"
dep "$lunar_demo" "$transfer"
dep "$lunar_demo" "$ground"
dep "$crewed_landing" "$landing"
dep "$crewed_landing" "$habitat"

# ------------------------------------------------ blocking work

dep "$ft_static" "$ft_stack"
dep "$ft_fts" "$ft_stack"
dep "$boostback" "$relight"
dep "$arm_seq" "$catch_bug"
dep "$turnaround" "$arm_seq"
dep "$turnaround" "$heat_bug"
dep "$flow" "$settle"
dep "$flow" "$latch"
dep "$latch" "$lidar"
dep "$flow" "$sensor_bug"
dep "$boiloff" "$mli"
dep "$legs" "$hazard"
dep "$crewed" "$uncrewed"
dep "$abort" "$hazard"
dep "$elevator" "$legs"
dep "$radiation" "$airlock"
dep "$flip" "$aerocapture"
dep "$cruise" "$boiloff"
dep "$subcool" "$valve_bug"
dep "$range" "$quench"
dep "$landing" "$transfer"
dep "$mars" "$transfer"

# ------------------------------------------------ status

for id in "$catch" "$catch_bug" "$relight" "$depot" "$mli" "$lidar" "$hazard" "$co2" "$valve_bug" "$sensor_bug" "$tankfarm"; do
	bd update "$id" --status=in_progress >/dev/null 2>&1
done
bd update "$flow" --status=blocked --notes="Waits for the second tanker ship to finish its static fire." >/dev/null 2>&1
bd update "$abort" --status=blocked --notes="Needs the descent hazard maps before the abort envelope can be drawn." >/dev/null 2>&1
bd update "$subcool" --status=blocked --notes="Blocked by the sticking oxygen fill valve." >/dev/null 2>&1
bd defer "$power" --until=2027-09-01 >/dev/null 2>&1
bd defer "$payload" --until=2027-12-01 >/dev/null 2>&1
bd defer "$docs" --until=2027-03-01 >/dev/null 2>&1

# ------------------------------------------------ comments

say tomas "$catch_bug" "Seen on two of three hot-stage tests. The starboard pin reads fine."
say kofi "$catch_bug" "The cells share one ground with the separation pyros. Moving them to the isolated ground bus."
say dario "$catch_bug" "Holding the arm qualification runs until this is fixed."
say ines "$boiloff" "First chamber run: 0.8%/day. Most of the heat comes in through the feed lines, not the dome."
say ines "$boiloff" "Feed-line standoffs halved it. Next run after the blankets arrive."
say kofi "$hazard" "Test flights over the quarry show the lidar misses boulders under 40 cm."
say yuki "$co2" "Amine beds hold 0.3% for three crew. Four crew needs the larger bed."
say dario "$valve_bug" "Valve seat shrinks faster than the body. Supplier sends a new seat material next week."
say mira "$flow" "This is the long pole for Lunar Demo. Weekly review until it moves."
say lena "$trajectory" "The next window opens late in the year; cruise is about six months either way."
say mira "$ft1" "Flight Test 1 flew the full profile. Booster splashed down on target."

# ------------------------------------------------ closed work

close_as dario "$ft_stack" --reason="Stacked in 3 h 10 min"
close_as tomas "$ft_static" --reason="All engines ran full duration"
close_as kofi "$ft_fts" --reason="Range safety signed off"
close_as mira "$flight" --reason="Flew the full profile"
close_as mira "$ft1" --reason="Released"
close_as tomas "$heat_bug" --reason="New tile retention clips"
close_as dario "$quench" --reason="Ten launches on one plate"
close_as lena "$trajectory" --reason="Trajectory chosen"
close_as yuki "$fire" --reason="Qualified in the cabin mock-up"
close_as dario "$weather" --reason="New station online"
close_as kofi "$lint" --reason="Builds clean on the new compiler"

mkdir -p "$here/.beads"
bd export >"$here/.beads/issues.jsonl"
echo "wrote $(wc -l <"$here/.beads/issues.jsonl" | tr -d ' ') issues to $here/.beads/issues.jsonl"
