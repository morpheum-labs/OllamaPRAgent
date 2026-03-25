package envx

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EnvOrDefault returns the first non-empty environment variable value parsed as T, or d.
func EnvOrDefault[T float64 | int | bool | string](d T, envs ...string) T {
	for _, env := range envs {
		if envVal := os.Getenv(env); envVal != "" {
			switch any(d).(type) {
			case int:
				if ival, err := strconv.Atoi(envVal); err == nil {
					return any(ival).(T)
				}
				fmt.Printf("failed to parse %s as an int. Using default %v\n", envVal, d)
			case float64:
				if fval, err := strconv.ParseFloat(envVal, 64); err == nil {
					return any(fval).(T)
				}
				fmt.Printf("failed to parse %s as a float. Using default %v\n", envVal, d)
			case bool:
				if bval, err := strconv.ParseBool(envVal); err == nil {
					return any(bval).(T)
				}
				fmt.Printf("failed to parse %s as a bool. Using default %v\n", envVal, d)
			case string:
				return any(envVal).(T)
			default:
				panic(fmt.Sprintf("unsupported type %T for EnvOrDefault", d))
			}
		}
	}

	return d
}

// OllamaURL resolves the Ollama base URL from common environment variables.
func OllamaURL() string {
	if url := os.Getenv("ORB_OLLAMA_URL"); url != "" {
		return url
	}
	if url := os.Getenv("OLLAMA_URL"); url != "" {
		return url
	}
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return "http://" + host
	}
	return "http://localhost:11434"
}

// PRNumber returns a PR number from ORB_PR_NUMBER or GITHUB_REF when set.
func PRNumber() int {
	if prNumStr := os.Getenv("ORB_PR_NUMBER"); prNumStr != "" {
		if prNum, err := strconv.Atoi(prNumStr); err == nil {
			return prNum
		}
		fmt.Printf("Failed to parse ORB_PR_NUMBER as an int\n")
	} else if ghRef := os.Getenv("GITHUB_REF"); ghRef != "" {
		parts := strings.Split(ghRef, "/")
		const prIndex = 2
		if len(parts) > prIndex {
			if prNum, err := strconv.Atoi(parts[prIndex]); err == nil {
				return prNum
			}
			fmt.Printf("Failed to parse PR number from GITHUB_REF as an int\n")
		}
	}
	return 0
}
