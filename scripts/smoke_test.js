/**
 * Homedash Smoke Test
 * Verified against --test-mode
 */

async function run() {
  tui.resize(120, 45);
  tui.waitForIdle(5000, 200);

  const dashboard = tui.text();
  
  // 1. Verify Header Mock Data
  if (!dashboard.includes("test-host")) throw new Error("Missing mock hostname");
  if (!dashboard.includes("12:00:00")) throw new Error("Missing frozen clock");
  if (!dashboard.includes("[test mode]")) throw new Error("Missing test mode indicator");

  // 2. Navigate to Detail View
  tui.pressKey("/");
  tui.sendText("nginx");
  tui.pressKey("enter");
  tui.waitForIdle(1000, 100);
  
  // After filtering "nginx", the core stack is expanded.
  // The selection might still be on the stack header. Move down to the container.
  tui.pressKey("j");
  tui.pressKey("enter"); // Open detail
  tui.waitForIdle(2000, 200);

  const detail = tui.text();
  
  // 3. Verify Detail Mock Data
  if (!detail.includes("/docker-entrypoint.sh nginx")) throw new Error("Missing mock command in detail");
  if (!detail.includes("Starting service...")) throw new Error("Missing mock logs in detail");

  console.log("SMOKE TEST SUCCESSFUL");
  tui.pressKey("q");
}

run().catch(err => {
  console.error("SMOKE TEST FAILED: " + err.message);
  tui.sendSignal("SIGKILL");
});
