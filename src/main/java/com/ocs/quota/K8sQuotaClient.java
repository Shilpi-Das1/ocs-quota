package com.ocs.quota;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

/**
 * Thin HTTP client for the Kubernetes ResourceQuota REST API.
 *
 * <p>Calls {@code /api/v1/namespaces/{ns}/resourcequotas/{name}} to read quota
 * hard limits and current usage from the OCP cluster.
 */
public class K8sQuotaClient {

    private final String    apiUrl;
    private final String    token;
    private final String    namespace;
    private final HttpClient http;
    private final ObjectMapper mapper = new ObjectMapper();

    public K8sQuotaClient(String apiUrl, String token, String namespace) {
        this.apiUrl    = apiUrl.replaceAll("/$", "");
        this.token     = token;
        this.namespace = namespace;
        this.http      = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(10))
                .build();
    }

    private JsonNode get(String path) throws IOException, InterruptedException {
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(apiUrl + path))
                .header("Authorization", "Bearer " + token)
                .header("Accept", "application/json")
                .GET()
                .build();
        HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
        if (resp.statusCode() >= 400) {
            throw new IOException(
                    "K8s API returned HTTP " + resp.statusCode() + " for " + path);
        }
        return mapper.readTree(resp.body());
    }

    /**
     * Return the hard storage limit ({@code requests.storage}) in GiB from a ResourceQuota.
     */
    public double getHardLimitGib(String quotaName) throws IOException, InterruptedException {
        String path = "/api/v1/namespaces/" + namespace + "/resourcequotas/" + quotaName;
        JsonNode data = get(path);
        String raw = data.path("spec").path("hard").path("requests.storage").asText("0");
        return parseQuantityGib(raw);
    }

    /**
     * Return the currently-used storage in GiB from a ResourceQuota status.
     */
    public double getUsedGib(String quotaName) throws IOException, InterruptedException {
        String path = "/api/v1/namespaces/" + namespace + "/resourcequotas/" + quotaName;
        JsonNode data = get(path);
        String raw = data.path("status").path("used").path("requests.storage").asText("0");
        return parseQuantityGib(raw);
    }

    /**
     * Parse a Kubernetes quantity string (e.g. {@code "10Gi"}, {@code "512Mi"}) to GiB.
     */
    static double parseQuantityGib(String quantity) {
        quantity = quantity.strip();
        if (quantity.endsWith("Gi")) return Double.parseDouble(quantity.substring(0, quantity.length() - 2));
        if (quantity.endsWith("Mi")) return Double.parseDouble(quantity.substring(0, quantity.length() - 2)) / 1024.0;
        if (quantity.endsWith("Ki")) return Double.parseDouble(quantity.substring(0, quantity.length() - 2)) / (1024.0 * 1024.0);
        if (quantity.endsWith("G"))  return Double.parseDouble(quantity.substring(0, quantity.length() - 1)) * 1e9 / (1024.0 * 1024.0 * 1024.0);
        return Double.parseDouble(quantity) / (1024.0 * 1024.0 * 1024.0);
    }
}
