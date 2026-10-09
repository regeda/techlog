// Illustrative: the Techlog API is a proposal; no hosted endpoint exists.
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Quickstart {
    static final String BASE = System.getenv().getOrDefault("TECHLOG_URL", "https://techlog.example.invalid");
    static final String TOKEN = System.getenv().getOrDefault("TECHLOG_TOKEN", "demo-token");
    static final HttpClient HTTP = HttpClient.newHttpClient();

    static String call(String method, String path, String json) throws Exception {
        HttpRequest.Builder b = HttpRequest.newBuilder(URI.create(BASE + path))
            .header("Authorization", "Bearer " + TOKEN)
            .header("Content-Type", "application/json");
        b.method(method, json == null ? HttpRequest.BodyPublishers.noBody() : HttpRequest.BodyPublishers.ofString(json));
        HttpResponse<String> res = HTTP.send(b.build(), HttpResponse.BodyHandlers.ofString());
        if (res.statusCode() >= 300) throw new RuntimeException(method + " " + path + " -> " + res.statusCode());
        return res.body();
    }

    public static void main(String[] args) throws Exception {
        // 1. Record a proposal, linked to the original problem report.
        String created = call("POST", "/v0/events", """
            {
              "schema_version": "0.1-draft",
              "type": "proposal.recorded",
              "time": "2026-03-10T11:30:00Z",
              "actor": { "kind": "ai_agent", "id": "agent:code-assistant", "on_behalf_of": "user:a.novak" },
              "subject": { "system": "promo-api", "component": "spin-service" },
              "status": "proposed",
              "summary": "Spin-to-win: one spin per new account, discount capped at 20%",
              "links": [{ "rel": "motivated_by", "target": "evt_01J9K2A0P002", "basis": "confirmed" }],
              "provenance": { "recorded_by": "agent:code-assistant", "method": "agent_submitted" }
            }
            """);
        Matcher m = Pattern.compile("\"id\"\\s*:\\s*\"([^\"]+)\"").matcher(created);
        if (!m.find()) throw new RuntimeException("no id in response: " + created);
        String id = m.group(1);
        System.out.println("recorded " + id);

        // 2. Attach verification evidence by reference.
        call("POST", "/v0/events/" + id + "/evidence", "{ \"kind\": \"test.report\", \"ref\": \"ci://run/90412\" }");

        // 3. Read recent history for the system.
        System.out.println(call("GET", "/v0/systems/promo-api/history?limit=5", null));
    }
}
