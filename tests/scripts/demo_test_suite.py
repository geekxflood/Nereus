#!/usr/bin/env python3
"""
Nereus Test Suite Demonstration Script

This script demonstrates the comprehensive testing capabilities of the Nereus
SNMP trap alerting system by running various test scenarios and generating
detailed reports.
"""

import os
import sys
import time
import json
import subprocess
import argparse
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Any

# Add the project root to Python path
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

class NereusTestRunner:
    """Comprehensive test runner for Nereus SNMP trap system."""
    
    def __init__(self, project_root: Path):
        self.project_root = project_root
        self.test_dir = project_root / "tests"
        self.reports_dir = project_root / "test-reports"
        self.start_time = datetime.now()
        
        # Ensure reports directory exists
        self.reports_dir.mkdir(exist_ok=True)
        
        # Test suite configuration
        self.test_suites = {
            "unit": {
                "name": "Unit Tests",
                "description": "Individual component testing",
                "command": ["make", "test-unit"],
                "timeout": 300,
                "critical": True
            },
            "integration": {
                "name": "Integration Tests", 
                "description": "Component interaction testing",
                "command": ["make", "test-integration"],
                "timeout": 600,
                "critical": True
            },
            "e2e": {
                "name": "End-to-End Tests",
                "description": "Complete workflow validation",
                "command": ["make", "test-e2e"],
                "timeout": 900,
                "critical": True
            },
            "performance": {
                "name": "Performance Tests",
                "description": "Load and benchmark testing",
                "command": ["make", "test-performance"],
                "timeout": 1800,
                "critical": False
            },
            "security": {
                "name": "Security Tests",
                "description": "Security vulnerability testing",
                "command": ["make", "test-security"],
                "timeout": 600,
                "critical": True
            },
            "chaos": {
                "name": "Chaos Engineering Tests",
                "description": "Resilience and failure testing",
                "command": ["make", "test-chaos"],
                "timeout": 1200,
                "critical": False
            }
        }
        
        self.results = {}
    
    def print_banner(self, title: str, char: str = "="):
        """Print a formatted banner."""
        width = 80
        print(f"\n{char * width}")
        print(f"{title:^{width}}")
        print(f"{char * width}\n")
    
    def print_status(self, message: str, status: str = "INFO"):
        """Print a status message with timestamp."""
        timestamp = datetime.now().strftime("%H:%M:%S")
        colors = {
            "INFO": "\033[94m",    # Blue
            "SUCCESS": "\033[92m", # Green
            "WARNING": "\033[93m", # Yellow
            "ERROR": "\033[91m",   # Red
            "RESET": "\033[0m"     # Reset
        }
        
        color = colors.get(status, colors["INFO"])
        reset = colors["RESET"]
        print(f"{color}[{timestamp}] {status}: {message}{reset}")
    
    def run_command(self, command: List[str], timeout: int = 300, cwd: Path = None) -> Dict[str, Any]:
        """Run a command and return results."""
        if cwd is None:
            cwd = self.test_dir
            
        self.print_status(f"Running: {' '.join(command)}")
        
        start_time = time.time()
        try:
            result = subprocess.run(
                command,
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=timeout
            )
            
            duration = time.time() - start_time
            
            return {
                "success": result.returncode == 0,
                "returncode": result.returncode,
                "stdout": result.stdout,
                "stderr": result.stderr,
                "duration": duration,
                "command": " ".join(command)
            }
            
        except subprocess.TimeoutExpired:
            duration = time.time() - start_time
            return {
                "success": False,
                "returncode": -1,
                "stdout": "",
                "stderr": f"Command timed out after {timeout} seconds",
                "duration": duration,
                "command": " ".join(command),
                "timeout": True
            }
        except Exception as e:
            duration = time.time() - start_time
            return {
                "success": False,
                "returncode": -1,
                "stdout": "",
                "stderr": str(e),
                "duration": duration,
                "command": " ".join(command),
                "error": str(e)
            }
    
    def setup_environment(self):
        """Set up the test environment."""
        self.print_banner("Setting Up Test Environment")
        
        # Check prerequisites
        self.print_status("Checking prerequisites...")
        
        # Check Go installation
        go_result = self.run_command(["go", "version"], timeout=10)
        if not go_result["success"]:
            self.print_status("Go is not installed or not in PATH", "ERROR")
            return False
        
        self.print_status(f"Go version: {go_result['stdout'].strip()}", "SUCCESS")
        
        # Check SNMP tools
        snmp_result = self.run_command(["snmptrap", "-h"], timeout=10)
        if not snmp_result["success"]:
            self.print_status("SNMP tools not available - some tests may fail", "WARNING")
        else:
            self.print_status("SNMP tools available", "SUCCESS")
        
        # Set up test environment
        setup_result = self.run_command(["make", "test-setup"], timeout=60)
        if not setup_result["success"]:
            self.print_status("Failed to set up test environment", "ERROR")
            self.print_status(setup_result["stderr"], "ERROR")
            return False
        
        self.print_status("Test environment set up successfully", "SUCCESS")
        return True
    
    def run_test_suite(self, suite_name: str, suite_config: Dict[str, Any]) -> Dict[str, Any]:
        """Run a specific test suite."""
        self.print_banner(f"Running {suite_config['name']}")
        self.print_status(suite_config["description"])
        
        result = self.run_command(
            suite_config["command"],
            timeout=suite_config["timeout"]
        )
        
        # Store result
        self.results[suite_name] = {
            **result,
            "name": suite_config["name"],
            "description": suite_config["description"],
            "critical": suite_config["critical"],
            "timestamp": datetime.now().isoformat()
        }
        
        if result["success"]:
            self.print_status(f"{suite_config['name']} completed successfully", "SUCCESS")
        else:
            status = "ERROR" if suite_config["critical"] else "WARNING"
            self.print_status(f"{suite_config['name']} failed", status)
            if result.get("timeout"):
                self.print_status(f"Test suite timed out after {suite_config['timeout']} seconds", status)
        
        self.print_status(f"Duration: {result['duration']:.2f} seconds")
        
        return result
    
    def run_all_tests(self, suites: List[str] = None):
        """Run all or specified test suites."""
        if suites is None:
            suites = list(self.test_suites.keys())
        
        self.print_banner("Nereus Comprehensive Test Suite")
        self.print_status(f"Starting test run at {self.start_time}")
        self.print_status(f"Test suites to run: {', '.join(suites)}")
        
        # Set up environment
        if not self.setup_environment():
            self.print_status("Environment setup failed - aborting test run", "ERROR")
            return False
        
        # Run test suites
        for suite_name in suites:
            if suite_name not in self.test_suites:
                self.print_status(f"Unknown test suite: {suite_name}", "WARNING")
                continue
            
            suite_config = self.test_suites[suite_name]
            self.run_test_suite(suite_name, suite_config)
        
        # Generate reports
        self.generate_reports()
        
        # Print summary
        self.print_summary()
        
        return self.get_overall_success()
    
    def generate_reports(self):
        """Generate test reports."""
        self.print_banner("Generating Test Reports")
        
        # Generate JSON report
        json_report = {
            "test_run": {
                "start_time": self.start_time.isoformat(),
                "end_time": datetime.now().isoformat(),
                "duration": (datetime.now() - self.start_time).total_seconds(),
                "suites_run": len(self.results),
                "overall_success": self.get_overall_success()
            },
            "results": self.results,
            "summary": self.get_summary_stats()
        }
        
        json_file = self.reports_dir / "test_results.json"
        with open(json_file, "w") as f:
            json.dump(json_report, f, indent=2)
        
        self.print_status(f"JSON report saved to {json_file}", "SUCCESS")
        
        # Generate HTML report
        self.generate_html_report(json_report)
        
        # Generate coverage report if available
        self.run_command(["make", "test-coverage"], timeout=120)
    
    def generate_html_report(self, data: Dict[str, Any]):
        """Generate HTML test report."""
        html_content = f"""
<!DOCTYPE html>
<html>
<head>
    <title>Nereus Test Results</title>
    <style>
        body {{ font-family: Arial, sans-serif; margin: 20px; }}
        .header {{ background: #f0f0f0; padding: 20px; border-radius: 5px; }}
        .success {{ color: green; }}
        .failure {{ color: red; }}
        .warning {{ color: orange; }}
        .suite {{ margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }}
        .duration {{ font-style: italic; color: #666; }}
        table {{ border-collapse: collapse; width: 100%; }}
        th, td {{ border: 1px solid #ddd; padding: 8px; text-align: left; }}
        th {{ background-color: #f2f2f2; }}
    </style>
</head>
<body>
    <div class="header">
        <h1>Nereus SNMP Trap System - Test Results</h1>
        <p><strong>Test Run:</strong> {data['test_run']['start_time']}</p>
        <p><strong>Duration:</strong> {data['test_run']['duration']:.2f} seconds</p>
        <p><strong>Overall Status:</strong> 
            <span class="{'success' if data['test_run']['overall_success'] else 'failure'}">
                {'PASSED' if data['test_run']['overall_success'] else 'FAILED'}
            </span>
        </p>
    </div>
    
    <h2>Test Suite Results</h2>
    <table>
        <tr>
            <th>Suite</th>
            <th>Status</th>
            <th>Duration</th>
            <th>Description</th>
        </tr>
"""
        
        for suite_name, result in data['results'].items():
            status_class = "success" if result['success'] else "failure"
            status_text = "PASSED" if result['success'] else "FAILED"
            
            html_content += f"""
        <tr>
            <td>{result['name']}</td>
            <td class="{status_class}">{status_text}</td>
            <td>{result['duration']:.2f}s</td>
            <td>{result['description']}</td>
        </tr>
"""
        
        html_content += """
    </table>
    
    <h2>Summary Statistics</h2>
    <ul>
"""
        
        summary = data['summary']
        html_content += f"""
        <li>Total Suites: {summary['total']}</li>
        <li>Passed: {summary['passed']}</li>
        <li>Failed: {summary['failed']}</li>
        <li>Critical Failures: {summary['critical_failures']}</li>
        <li>Success Rate: {summary['success_rate']:.1f}%</li>
"""
        
        html_content += """
    </ul>
</body>
</html>
"""
        
        html_file = self.reports_dir / "test_results.html"
        with open(html_file, "w") as f:
            f.write(html_content)
        
        self.print_status(f"HTML report saved to {html_file}", "SUCCESS")
    
    def get_summary_stats(self) -> Dict[str, Any]:
        """Get summary statistics."""
        total = len(self.results)
        passed = sum(1 for r in self.results.values() if r['success'])
        failed = total - passed
        critical_failures = sum(1 for r in self.results.values() if not r['success'] and r['critical'])
        
        return {
            "total": total,
            "passed": passed,
            "failed": failed,
            "critical_failures": critical_failures,
            "success_rate": (passed / total * 100) if total > 0 else 0
        }
    
    def get_overall_success(self) -> bool:
        """Determine overall test success."""
        # Success if all critical tests pass
        for result in self.results.values():
            if result['critical'] and not result['success']:
                return False
        return True
    
    def print_summary(self):
        """Print test summary."""
        self.print_banner("Test Summary")
        
        summary = self.get_summary_stats()
        overall_success = self.get_overall_success()
        
        self.print_status(f"Total test suites: {summary['total']}")
        self.print_status(f"Passed: {summary['passed']}", "SUCCESS")
        self.print_status(f"Failed: {summary['failed']}", "ERROR" if summary['failed'] > 0 else "INFO")
        self.print_status(f"Critical failures: {summary['critical_failures']}", "ERROR" if summary['critical_failures'] > 0 else "INFO")
        self.print_status(f"Success rate: {summary['success_rate']:.1f}%")
        
        total_duration = (datetime.now() - self.start_time).total_seconds()
        self.print_status(f"Total duration: {total_duration:.2f} seconds")
        
        if overall_success:
            self.print_status("Overall result: PASSED", "SUCCESS")
        else:
            self.print_status("Overall result: FAILED", "ERROR")
        
        # Print individual suite results
        print("\nDetailed Results:")
        for suite_name, result in self.results.items():
            status = "PASSED" if result['success'] else "FAILED"
            status_type = "SUCCESS" if result['success'] else ("ERROR" if result['critical'] else "WARNING")
            self.print_status(f"{result['name']}: {status} ({result['duration']:.2f}s)", status_type)
    
    def cleanup(self):
        """Clean up test environment."""
        self.print_status("Cleaning up test environment...")
        self.run_command(["make", "test-clean"], timeout=60)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(description="Nereus Test Suite Demonstration")
    parser.add_argument(
        "--suites",
        nargs="+",
        choices=["unit", "integration", "e2e", "performance", "security", "chaos"],
        help="Test suites to run (default: all)"
    )
    parser.add_argument(
        "--quick",
        action="store_true",
        help="Run only critical test suites (unit, integration, e2e, security)"
    )
    
    args = parser.parse_args()
    
    # Determine which suites to run
    if args.quick:
        suites = ["unit", "integration", "e2e", "security"]
    elif args.suites:
        suites = args.suites
    else:
        suites = None  # Run all suites
    
    # Create test runner
    runner = NereusTestRunner(project_root)
    
    try:
        # Run tests
        success = runner.run_all_tests(suites)
        
        # Exit with appropriate code
        sys.exit(0 if success else 1)
        
    except KeyboardInterrupt:
        runner.print_status("Test run interrupted by user", "WARNING")
        sys.exit(130)
    except Exception as e:
        runner.print_status(f"Test run failed with error: {e}", "ERROR")
        sys.exit(1)
    finally:
        runner.cleanup()


if __name__ == "__main__":
    main()
