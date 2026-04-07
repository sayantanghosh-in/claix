#!/usr/bin/env python3
"""
Generate fake Claude Code session data for demo screenshots and GIFs.

Usage:
    python3 scripts/generate-demo-data.py

This creates ~/.claude-demo/ with realistic but funny session data.
To use: CLAUDE_HOME=~/.claude-demo claix

To delete: rm -rf ~/.claude-demo
"""

import json
import os
import uuid
from datetime import datetime, timedelta

DEMO_HOME = os.path.expanduser("~/.claude-demo")
PROJECTS_DIR = os.path.join(DEMO_HOME, "projects")

# =====================================================================
# FAKE SESSIONS — funny, catchy, meme-worthy
# =====================================================================

SESSIONS = [
    {
        "project": "-Users-demo-projects-yolo-deploy",
        "branch": "fix-the-fix-that-fixed-nothing",
        "slug": "panicking-caffeinated-gopher",
        "version": "2.1.92",
        "days_ago": 0,
        "hour": 8,
        "user_messages": [
            "the deploy script works on my machine but production is literally on fire. the CEO just sent me a slack message that just says '🔥🔥🔥'. please help.",
            "ok so apparently someone committed a console.log('lol') that breaks the JSON parser in production. i am not naming names but it was me.",
            "can you also add a pre-commit hook so i never do this again? my dignity depends on it.",
        ],
        "assistant_messages": [
            "I can see the issue! There's a `console.log('lol')` on line 42 of `server.js` that's being included in the API response body, breaking the JSON parser for every client. Let me fix this immediately.",
            "I've removed the rogue console.log and added a `.eslintrc` rule to catch console statements. I've also added a pre-commit hook using husky that runs eslint before every commit. Your dignity is safe now.",
            "I've also added a GitHub Action that runs on PRs to catch this in code review. Here's the PR with all fixes.",
        ],
        "files_read": [
            "/Users/demo/projects/yolo-deploy/server.js",
            "/Users/demo/projects/yolo-deploy/package.json",
            "/Users/demo/projects/yolo-deploy/.github/workflows/ci.yml",
            "/Users/demo/projects/yolo-deploy/deploy.sh",
        ],
        "files_edited": [
            "/Users/demo/projects/yolo-deploy/server.js",
            "/Users/demo/projects/yolo-deploy/.eslintrc.json",
            "/Users/demo/projects/yolo-deploy/.husky/pre-commit",
            "/Users/demo/projects/yolo-deploy/.github/workflows/lint.yml",
        ],
        "prs": [
            {"number": 42, "repo": "acme-corp/yolo-deploy", "url": "https://github.com/acme-corp/yolo-deploy/pull/42"},
        ],
    },
    {
        "project": "-Users-demo-projects-taco-tuesday-api",
        "branch": "feature/extra-guac",
        "slug": "hungry-dancing-burrito",
        "version": "2.1.92",
        "days_ago": 0,
        "hour": 14,
        "user_messages": [
            "we're building a taco ordering API and i need you to add a 'extra guacamole' option. this is the most important feature our startup has ever shipped.",
            "the guac pricing logic needs to account for 'avocado market fluctuations'. yes this is real. our PM insists.",
            "perfect. now add a webhook that pings our slack channel #taco-emergencies whenever guac supply drops below 50%",
        ],
        "assistant_messages": [
            "I'll add the extra guacamole option to the order schema. This is clearly mission-critical functionality. Let me update the API endpoint and the pricing calculator.",
            "I've implemented a dynamic guac pricing engine that fetches real-time avocado prices. The `GuacFluctuationService` adjusts prices based on supply levels. I added sensible defaults in case the avocado API is down.",
            "Webhook configured! The #taco-emergencies channel will receive alerts with a custom 🥑 emoji when supplies are low. I've also added a Grafana dashboard for real-time guac monitoring.",
        ],
        "files_read": [
            "/Users/demo/projects/taco-tuesday-api/src/routes/orders.ts",
            "/Users/demo/projects/taco-tuesday-api/src/models/taco.ts",
            "/Users/demo/projects/taco-tuesday-api/src/services/pricing.ts",
        ],
        "files_edited": [
            "/Users/demo/projects/taco-tuesday-api/src/routes/orders.ts",
            "/Users/demo/projects/taco-tuesday-api/src/models/taco.ts",
            "/Users/demo/projects/taco-tuesday-api/src/services/guac-pricing.ts",
            "/Users/demo/projects/taco-tuesday-api/src/webhooks/taco-alerts.ts",
            "/Users/demo/projects/taco-tuesday-api/src/dashboards/guac-monitor.json",
        ],
        "prs": [
            {"number": 69, "repo": "taco-corp/taco-tuesday-api", "url": "https://github.com/taco-corp/taco-tuesday-api/pull/69"},
        ],
    },
    {
        "project": "-Users-demo-projects-ai-cat-detector",
        "branch": "main",
        "slug": "curious-napping-kitten",
        "version": "2.1.90",
        "days_ago": 1,
        "hour": 22,
        "user_messages": [
            "i trained a neural network to detect cats in photos but it keeps classifying my boss's face as a persian cat. this is both hilarious and a career-ending bug.",
            "the model has 99.7% accuracy on cats but apparently my boss has very cat-like features. how do we fix this without retraining from scratch?",
        ],
        "assistant_messages": [
            "That's... an interesting edge case. The issue is likely that your training data has a bias toward round faces with certain features. Let me look at the model architecture and the preprocessing pipeline.",
            "I've added a 'human face pre-filter' step that runs a lightweight face detection model before the cat classifier. If a human face is detected with >80% confidence, it skips cat classification entirely. Your boss is safe. I've also added this to the test suite with a note: 'Ensure humans are not classified as cats.'",
        ],
        "files_read": [
            "/Users/demo/projects/ai-cat-detector/model/classifier.py",
            "/Users/demo/projects/ai-cat-detector/preprocessing/pipeline.py",
            "/Users/demo/projects/ai-cat-detector/tests/test_classifier.py",
        ],
        "files_edited": [
            "/Users/demo/projects/ai-cat-detector/preprocessing/face_filter.py",
            "/Users/demo/projects/ai-cat-detector/model/classifier.py",
            "/Users/demo/projects/ai-cat-detector/tests/test_classifier.py",
        ],
        "prs": [],
    },
    {
        "project": "-Users-demo-projects-startup-landing",
        "branch": "make-it-pop",
        "slug": "vibrant-pixelated-unicorn",
        "version": "2.1.92",
        "days_ago": 2,
        "hour": 10,
        "user_messages": [
            "my designer said the landing page needs to 'pop more' and 'feel more disruptive'. i have no idea what that means. please make it pop.",
            "she also wants the hero section to have 'big startup energy'. i am quoting directly.",
            "ok she loves it! but now she wants a dark mode toggle that transitions with a 'sunset animation'. i wish i was joking.",
        ],
        "assistant_messages": [
            "I'll make it pop! Adding gradient backgrounds, larger typography with a bold sans-serif font, animated entrance effects, and a floating CTA button with a subtle pulse animation. Maximum pop achieved.",
            "Big startup energy deployed! I've added an oversized headline with a typewriter effect, a 3D animated product mockup, social proof counters that animate on scroll, and a 'Backed by Y Combinator*' badge (*not actually backed by Y Combinator).",
            "The sunset dark mode toggle is done. When you flip the toggle, the background transitions through golden hour colors (warm orange → deep purple → dark navy) over 2 seconds. It's genuinely beautiful. Your designer has taste.",
        ],
        "files_read": [
            "/Users/demo/projects/startup-landing/src/App.tsx",
            "/Users/demo/projects/startup-landing/src/components/Hero.tsx",
            "/Users/demo/projects/startup-landing/src/styles/theme.css",
        ],
        "files_edited": [
            "/Users/demo/projects/startup-landing/src/components/Hero.tsx",
            "/Users/demo/projects/startup-landing/src/components/DarkModeToggle.tsx",
            "/Users/demo/projects/startup-landing/src/styles/theme.css",
            "/Users/demo/projects/startup-landing/src/styles/animations.css",
            "/Users/demo/projects/startup-landing/src/styles/sunset-transition.css",
        ],
        "prs": [
            {"number": 7, "repo": "unicorn-inc/startup-landing", "url": "https://github.com/unicorn-inc/startup-landing/pull/7"},
            {"number": 8, "repo": "unicorn-inc/startup-landing", "url": "https://github.com/unicorn-inc/startup-landing/pull/8"},
        ],
    },
    {
        "project": "-Users-demo-projects-todo-app-the-sequel",
        "branch": "refactor/yet-another-rewrite",
        "slug": "ambitious-overthinking-dev",
        "version": "2.1.87",
        "days_ago": 3,
        "hour": 3,
        "user_messages": [
            "ok hear me out. i know this is my 4th todo app rewrite this year but THIS time we're doing it right. microservices, event sourcing, CQRS, kubernetes. for a todo app.",
            "i also need real-time collaboration, offline-first sync, and end-to-end encryption. for adding 'buy milk' to a list.",
        ],
        "assistant_messages": [
            "I admire the ambition! Let me set up the microservices architecture. We'll have: TodoService, UserService, NotificationService, SyncService, EncryptionService, and a Kubernetes orchestrator. For a todo app. Let's do this.",
            "I've scaffolded 6 microservices with gRPC communication, an event store using PostgreSQL, a CQRS read model, and a Helm chart for k8s deployment. Total infrastructure cost estimate: $847/month. To track 'buy milk'. But it will be the most architecturally sound todo app ever built.",
        ],
        "files_read": [
            "/Users/demo/projects/todo-app-the-sequel/docker-compose.yml",
            "/Users/demo/projects/todo-app-the-sequel/services/todo-service/main.go",
        ],
        "files_edited": [
            "/Users/demo/projects/todo-app-the-sequel/docker-compose.yml",
            "/Users/demo/projects/todo-app-the-sequel/services/todo-service/main.go",
            "/Users/demo/projects/todo-app-the-sequel/services/user-service/main.go",
            "/Users/demo/projects/todo-app-the-sequel/services/notification-service/main.go",
            "/Users/demo/projects/todo-app-the-sequel/services/sync-service/main.go",
            "/Users/demo/projects/todo-app-the-sequel/services/encryption-service/main.go",
            "/Users/demo/projects/todo-app-the-sequel/k8s/helm/values.yaml",
            "/Users/demo/projects/todo-app-the-sequel/k8s/helm/Chart.yaml",
        ],
        "prs": [],
    },
    {
        "project": "-Users-demo-projects-yolo-deploy",
        "branch": "hotfix/friday-deploy-regret",
        "slug": "reckless-friday-deployer",
        "version": "2.1.92",
        "days_ago": 4,
        "hour": 17,
        "user_messages": [
            "i deployed to production on a friday at 5pm. yes i know. please don't judge me. anyway the database migration failed halfway through and now half the users have the new schema and half don't.",
            "can you write a rollback script that handles both states? i need to fix this before monday. my weekend depends on it.",
        ],
        "assistant_messages": [
            "No judgment. We've all been there. (But also: never deploy on Friday at 5pm.) Let me look at the migration state.",
            "I've written a safe rollback script that: 1) Detects which schema version each user row has, 2) Rolls back only the partially-migrated rows, 3) Verifies data integrity after rollback, 4) Sends you a Slack notification when it's done. Your weekend is saved. I've also added a CI check that blocks deploys after 4pm on Fridays.",
        ],
        "files_read": [
            "/Users/demo/projects/yolo-deploy/migrations/20260401_add_columns.sql",
            "/Users/demo/projects/yolo-deploy/scripts/check-migration-state.sh",
        ],
        "files_edited": [
            "/Users/demo/projects/yolo-deploy/scripts/emergency-rollback.sh",
            "/Users/demo/projects/yolo-deploy/.github/workflows/deploy-guard.yml",
        ],
        "prs": [
            {"number": 43, "repo": "acme-corp/yolo-deploy", "url": "https://github.com/acme-corp/yolo-deploy/pull/43"},
        ],
    },
    {
        "project": "-Users-demo-projects-dating-app-for-dogs",
        "branch": "feature/bark-detection",
        "slug": "friendly-tail-wagging-pup",
        "version": "2.1.90",
        "days_ago": 5,
        "hour": 11,
        "user_messages": [
            "we're building a dating app for dogs. yes, really. investors loved the pitch. we need a bark-to-text feature so dogs can write their own bios.",
            "the bark classifier needs to distinguish between 'excited bark' (swipe right), 'aggressive bark' (block user), and 'confused bark' (show more profiles).",
        ],
        "assistant_messages": [
            "This is the most innovative use of AI I've seen this quarter. Let me build the bark-to-text pipeline using a fine-tuned audio classifier.",
            "I've implemented the BarkAnalyzer with three modes: EXCITED (maps to 'Looking for a pawsome time! 🐾'), AGGRESSIVE (maps to 'Not interested, go fetch somewhere else'), and CONFUSED (maps to 'Still sniffing around'). Accuracy is surprisingly high at 87%. Also added a 'tail wag intensity' metric from the accelerometer for the premium tier.",
        ],
        "files_read": [
            "/Users/demo/projects/dating-app-for-dogs/src/audio/bark-classifier.py",
            "/Users/demo/projects/dating-app-for-dogs/src/models/dog-profile.ts",
        ],
        "files_edited": [
            "/Users/demo/projects/dating-app-for-dogs/src/audio/bark-classifier.py",
            "/Users/demo/projects/dating-app-for-dogs/src/audio/bark-to-text.py",
            "/Users/demo/projects/dating-app-for-dogs/src/models/dog-profile.ts",
        ],
        "prs": [
            {"number": 1, "repo": "woofwoof-inc/dating-app-for-dogs", "url": "https://github.com/woofwoof-inc/dating-app-for-dogs/pull/1"},
        ],
    },
    {
        "project": "-Users-demo-projects-taco-tuesday-api",
        "branch": "bugfix/infinite-salsa-loop",
        "slug": "dizzy-spinning-taco",
        "version": "2.1.87",
        "days_ago": 7,
        "hour": 9,
        "user_messages": [
            "there's a bug where ordering salsa triggers a webhook that orders more salsa which triggers another webhook. we accidentally ordered 47,000 jars of salsa from the supplier API. the warehouse is asking questions.",
        ],
        "assistant_messages": [
            "That's a classic infinite recursion bug — the salsa webhook calls itself. I've added a deduplication check using an idempotency key on each webhook call, and a circuit breaker that stops after 3 salsa orders per minute. I've also cancelled the 46,997 excess salsa orders via the supplier's batch cancellation API. You're welcome.",
        ],
        "files_read": [
            "/Users/demo/projects/taco-tuesday-api/src/webhooks/order-handler.ts",
            "/Users/demo/projects/taco-tuesday-api/src/services/supplier-api.ts",
        ],
        "files_edited": [
            "/Users/demo/projects/taco-tuesday-api/src/webhooks/order-handler.ts",
            "/Users/demo/projects/taco-tuesday-api/src/middleware/idempotency.ts",
            "/Users/demo/projects/taco-tuesday-api/src/middleware/circuit-breaker.ts",
        ],
        "prs": [
            {"number": 70, "repo": "taco-corp/taco-tuesday-api", "url": "https://github.com/taco-corp/taco-tuesday-api/pull/70"},
        ],
    },
    {
        "project": "-Users-demo-projects-ai-cat-detector",
        "branch": "feature/multi-breed-support",
        "slug": "sophisticated-monocle-cat",
        "version": "2.1.92",
        "days_ago": 0,
        "hour": 16,
        "user_messages": [
            "the cat detector works great but it classifies every cat as 'cat'. investors want breed detection — apparently knowing it's a 'Maine Coon' is worth 3x the valuation.",
        ],
        "assistant_messages": [
            "I've added a breed classification layer on top of the existing detector. It now identifies 40 breeds with 91% accuracy. The model was surprisingly confident that the office plant is a 'Scottish Fold' though, so I've added a minimum cat-confidence threshold before breed classification kicks in. Also added a breed info tooltip in the UI that shows fun facts like 'Maine Coons are basically small lions that live in your house.'",
        ],
        "files_read": [
            "/Users/demo/projects/ai-cat-detector/model/classifier.py",
            "/Users/demo/projects/ai-cat-detector/data/breed-labels.json",
        ],
        "files_edited": [
            "/Users/demo/projects/ai-cat-detector/model/breed_classifier.py",
            "/Users/demo/projects/ai-cat-detector/data/breed-labels.json",
            "/Users/demo/projects/ai-cat-detector/api/routes.py",
        ],
        "prs": [
            {"number": 13, "repo": "pettech/ai-cat-detector", "url": "https://github.com/pettech/ai-cat-detector/pull/13"},
        ],
    },
    # Active session (last message from user — no response yet)
    {
        "project": "-Users-demo-projects-startup-landing",
        "branch": "feat/ai-chatbot",
        "slug": "optimistic-typing-cursor",
        "version": "2.1.92",
        "days_ago": 0,
        "hour": 7,
        "user_messages": [
            "add an AI chatbot to the landing page that answers questions about our product. but here's the thing — we don't have a product yet. make the chatbot confidently vague.",
        ],
        "assistant_messages": [],  # No response = active session
        "files_read": [],
        "files_edited": [],
        "prs": [],
    },
    # Empty sessions
    {
        "project": "-Users-demo-projects-blockchain-for-pets",
        "branch": "",
        "slug": "",
        "version": "",
        "days_ago": 10,
        "hour": 14,
        "user_messages": [],
        "assistant_messages": [],
        "files_read": [],
        "files_edited": [],
        "prs": [],
    },
]

# =====================================================================
# GENERATE .jsonl FILES
# =====================================================================

def make_timestamp(days_ago, hour, minute=0, second=0):
    dt = datetime.now() - timedelta(days=days_ago)
    dt = dt.replace(hour=hour, minute=minute, second=second, microsecond=0)
    return dt.strftime("%Y-%m-%dT%H:%M:%S.000Z")

def generate_session(session_data):
    project_dir = os.path.join(PROJECTS_DIR, session_data["project"])
    os.makedirs(project_dir, exist_ok=True)

    session_id = str(uuid.uuid4())
    jsonl_path = os.path.join(project_dir, f"{session_id}.jsonl")

    lines = []
    ts_offset = 0  # seconds offset from start

    base_ts = make_timestamp(session_data["days_ago"], session_data["hour"])

    # Queue operation: enqueue
    lines.append(json.dumps({
        "type": "queue-operation",
        "operation": "enqueue",
        "timestamp": base_ts,
        "sessionId": session_id,
    }))
    ts_offset += 1

    # Queue operation: dequeue
    lines.append(json.dumps({
        "type": "queue-operation",
        "operation": "dequeue",
        "timestamp": make_timestamp(session_data["days_ago"], session_data["hour"], 0, ts_offset),
        "sessionId": session_id,
    }))
    ts_offset += 1

    # Interleave user and assistant messages
    user_msgs = session_data["user_messages"]
    assistant_msgs = session_data["assistant_messages"]
    max_pairs = max(len(user_msgs), len(assistant_msgs))

    for i in range(max_pairs):
        if i < len(user_msgs):
            ts_offset += 5
            lines.append(json.dumps({
                "type": "user",
                "timestamp": make_timestamp(session_data["days_ago"], session_data["hour"], ts_offset // 60, ts_offset % 60),
                "sessionId": session_id,
                "gitBranch": session_data["branch"],
                "version": session_data["version"],
                "isMeta": False,
                "isSidechain": False,
                "userType": "external",
                "cwd": "/Users/demo/projects/" + session_data["project"].replace("-Users-demo-projects-", ""),
                "uuid": str(uuid.uuid4()),
                "message": {
                    "role": "user",
                    "content": [{"type": "text", "text": user_msgs[i]}]
                },
            }))

        if i < len(assistant_msgs):
            ts_offset += 15
            # Build content blocks: text + tool_use for files
            content = [{"type": "text", "text": assistant_msgs[i]}]

            # Add file read tool calls
            if i == 0:  # First response reads files
                for fp in session_data["files_read"]:
                    content.append({
                        "type": "tool_use",
                        "id": f"toolu_{uuid.uuid4().hex[:24]}",
                        "name": "Read",
                        "input": {"file_path": fp},
                    })

            # Add file edit tool calls
            if i == len(assistant_msgs) - 1:  # Last response edits files
                for fp in session_data["files_edited"]:
                    content.append({
                        "type": "tool_use",
                        "id": f"toolu_{uuid.uuid4().hex[:24]}",
                        "name": "Edit",
                        "input": {"file_path": fp},
                    })

            slug = session_data.get("slug", "")
            lines.append(json.dumps({
                "type": "assistant",
                "timestamp": make_timestamp(session_data["days_ago"], session_data["hour"], ts_offset // 60, ts_offset % 60),
                "sessionId": session_id,
                "gitBranch": session_data["branch"],
                "version": session_data["version"],
                "slug": slug,
                "isSidechain": False,
                "userType": "external",
                "cwd": "/Users/demo/projects/" + session_data["project"].replace("-Users-demo-projects-", ""),
                "uuid": str(uuid.uuid4()),
                "requestId": f"req_{uuid.uuid4().hex[:24]}",
                "message": {
                    "role": "assistant",
                    "content": content,
                },
            }))

    # Add PR links
    for pr in session_data["prs"]:
        ts_offset += 5
        lines.append(json.dumps({
            "type": "pr-link",
            "timestamp": make_timestamp(session_data["days_ago"], session_data["hour"], ts_offset // 60, ts_offset % 60),
            "sessionId": session_id,
            "prNumber": pr["number"],
            "prRepository": pr["repo"],
            "prUrl": pr["url"],
        }))

    with open(jsonl_path, "w") as f:
        f.write("\n".join(lines) + "\n")

    print(f"  Created: {session_data['project'].replace('-Users-demo-projects-', '')} ({session_id[:8]})")

# Also create a fake stats-cache.json
def generate_stats_cache():
    stats = {
        "version": 2,
        "lastComputedDate": datetime.now().strftime("%Y-%m-%d"),
        "dailyActivity": [],
        "dailyModelTokens": [],
    }

    for i in range(28):
        dt = datetime.now() - timedelta(days=27 - i)
        date_str = dt.strftime("%Y-%m-%d")

        # Skip some days for realistic gaps
        if i % 7 in (5, 6) and i > 5:  # Weekends are quieter
            msgs = max(0, 50 - abs(i - 14) * 3)
        else:
            msgs = 100 + (i * 30) + (hash(date_str) % 200)

        tools = msgs // 3
        sessions = max(1, msgs // 100)

        stats["dailyActivity"].append({
            "date": date_str,
            "messageCount": msgs,
            "sessionCount": sessions,
            "toolCallCount": tools,
        })

        opus_tokens = msgs * 50 + (hash(date_str) % 5000)
        sonnet_tokens = msgs * 10
        stats["dailyModelTokens"].append({
            "date": date_str,
            "tokensByModel": {
                "claude-opus-4-6": opus_tokens,
                "claude-sonnet-4-6": sonnet_tokens,
            },
        })

    stats_path = os.path.join(DEMO_HOME, "stats-cache.json")
    with open(stats_path, "w") as f:
        json.dump(stats, f, indent=2)
    print(f"  Created: stats-cache.json")


if __name__ == "__main__":
    print("Generating demo data at ~/.claude-demo/\n")

    # Clean existing demo data
    import shutil
    if os.path.exists(PROJECTS_DIR):
        shutil.rmtree(PROJECTS_DIR)
    os.makedirs(PROJECTS_DIR)

    for session in SESSIONS:
        generate_session(session)

    generate_stats_cache()

    print(f"\nDone! {len(SESSIONS)} sessions created.")
    print(f"\nTo use:")
    print(f"  CLAUDE_HOME=~/.claude-demo claix")
    print(f"\nTo delete:")
    print(f"  rm -rf ~/.claude-demo")
