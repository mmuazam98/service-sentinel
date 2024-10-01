# Service Sentinel

![Go](https://img.shields.io/badge/Language-Go-blue.svg)
![License](https://img.shields.io/badge/License-MIT-green.svg)

**Service Sentinel** is an open-source service health checker built with Go. It monitors the health status of your microservices by periodically checking their health endpoints and logging the results. This project aims to provide an easy way to ensure that your services are running smoothly and alert you to any issues.

## Features

- Periodic health checks of specified services
- Logs results of health checks with color-coded statuses
- Customizable interval for health checks
- Easy to configure using a YAML file
- Support for healthy and unhealthy service statuses

## Getting Started

### Prerequisites

- [Go](https://golang.org/doc/install) installed on your machine
- A YAML file for service configuration

### Installation

1. Clone the repository:

```bash
git clone https://github.com/mmuazam98/service-sentinel.git
cd service-sentinel
```

2. Install dependencies (if any):

```bash
go mod tidy
```

3. Update the config/config.yaml file to define the services you want to monitor. Below is an example:

```yaml
services:
  - name: "GitHub API"
    url: "https://api.github.com"
  - name: "JSONPlaceholder"
    url: "https://jsonplaceholder.typicode.com/posts"
  - name: "OpenWeatherMap"
    url: "https://api.openweathermap.org/data/2.5/weather?q=London&appid=YOUR_API_KEY"
  - name: "Dog API"
    url: "https://dog.ceo/api/breeds/image/random"
  - name: "Public APIs"
    url: "https://api.publicapis.org/entries"
  - name: "Unhealthy 404"
    url: "https://httpstat.us/404"
  - name: "Unhealthy 500"
    url: "https://httpstat.us/500"

alert_webhook_url: "https://webhook.site/..."
slack_webhook_url: "https://hooks.slack.com/services/.../..."
```

### Configuration Notes

- Update the `alert_webhook_url` and/or `slack_webhook_url` fields with valid webhook URLs to receive notifications.
- Optionally, you can set the environment variables `ALERT_WEBHOOK_URL` and `SLACK_WEBHOOK_URL` to override the values in the configuration file.

## Running the Application

To start the health checker, run the following command:

```bash
go run cmd/main.go --interval 1
```

Replace 1 with your desired interval in minutes for health checks.

## Example Output

You will see output similar to the following in your console:

```bash
Loaded 7 services from config:
  - GitHub API: https://api.github.com
  - JSONPlaceholder: https://jsonplaceholder.typicode.com/posts
  - OpenWeatherMap: https://api.openweathermap.org/data/2.5/weather?q=London&appid=YOUR_API_KEY
  - Dog API: https://dog.ceo/api/breeds/image/random
  - Public APIs: https://api.publicapis.org/entries
  - Unhealthy 404: https://httpstat.us/404
  - Unhealthy 500: https://httpstat.us/500
[INFO] 2024-10-01 12:29:33: Service Sentinel running every 1 minutes...
[INFO] 2024-10-01 12:29:33: Starting health checks...
[INFO] 2024-10-01 12:29:34: Service 'JSONPlaceholder' (https://jsonplaceholder.typicode.com/posts) is Healthy (Response time: 349.52725ms)
[WARN] 2024-10-01 12:29:34: Service 'OpenWeatherMap' (https://api.openweathermap.org/data/2.5/weather?q=London&appid=YOUR_API_KEY) returned status code 401 (Response time: 395.360458ms)
[INFO] 2024-10-01 12:29:34: Service 'GitHub API' (https://api.github.com) is Healthy (Response time: 420.892958ms)
[INFO] 2024-10-01 12:29:34: Service 'Dog API' (https://dog.ceo/api/breeds/image/random) is Healthy (Response time: 702.446333ms)
[WARN] 2024-10-01 12:29:35: Service 'Unhealthy 500' (https://httpstat.us/500) returned status code 500 (Response time: 1.394005917s)
[WARN] 2024-10-01 12:29:35: Service 'Unhealthy 404' (https://httpstat.us/404) returned status code 404 (Response time: 1.418692042s)
[ERROR] 2024-10-01 12:29:38: Service 'Public APIs' (https://api.publicapis.org/entries) is Unhealthy! Error: Get "https://api.publicapis.org/entries": context deadline exceeded (Client.Timeout exceeded while awaiting headers) (Response time: 5.001417208s)
[INFO] 2024-10-01 12:29:38: Health check round completed.
```

## Testing

To run tests for the health check functionality, use the following command:

```
go test ./pkg/checker -v
```

## Contributing

Contributions are welcome! If you have suggestions for improvements or new features, please open an issue or submit a pull request.

- Fork the repository
- Create a new branch (git checkout -b feature-branch)
- Make your changes
- Commit your changes (git commit -am 'Add new feature')
- Push to the branch (git push origin feature-branch)
- Create a new Pull Request

## License

This project is licensed under the MIT License. See the LICENSE file for details.
