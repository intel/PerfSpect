#!/usr/bin/env python3
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

"""
Compare metric values between two CSV files.
Used to compare EMON system summary metrics with PerfSpect system summary metrics.
"""

import sys
import csv
import argparse
from typing import Dict, List, Tuple


def read_csv(filepath: str) -> tuple[Dict[str, Dict[str, str]], List[str]]:
    """Read CSV file and return dict of metric name -> metric data, and list of metric names in order."""
    metrics = {}
    metric_order = []
    with open(filepath, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            # Skip empty rows
            if not row:
                continue
            
            # Try multiple column name patterns
            # Handle cases like "(Metric post processor 5.18.0) name (sample #1 - #304)"
            name = None
            for key in row.keys():
                if 'name' in key.lower() or 'metric' in key.lower():
                    name = row[key]
                    break
            
            if not name:
                continue
            
            # Remove 'metric_' prefix if present
            if name.startswith('metric_'):
                name = name[7:]  # Remove 'metric_' prefix
            metrics[name] = row
            metric_order.append(name)
    return metrics, metric_order


def parse_value(value_str: str) -> float:
    """Parse a numeric value from string, handling scientific notation."""
    try:
        return float(value_str)
    except (ValueError, TypeError):
        return None


def calculate_percent_difference(val1: float, val2: float) -> float:
    """Calculate percent difference: ((val2 - val1) / val1) * 100."""
    if val1 == 0:
        if val2 == 0:
            return 0.0
        return float('inf')
    return ((val2 - val1) / val1) * 100


def categorize_difference(pct_diff: float) -> Tuple[str, str]:
    """Categorize the percent difference and return (status, symbol)."""
    abs_diff = abs(pct_diff)
    if abs_diff < 5:
        return ("Excellent", "✓")
    elif abs_diff < 10:
        return ("Good", "✓")
    elif abs_diff < 25:
        return ("Moderate", "")
    elif abs_diff < 50:
        return ("Large", "⚠️")
    else:
        return ("Critical", "⚠️")


def print_header():
    """Print the header section."""
    print("=" * 120)
    print("METRIC COMPARISON ANALYSIS")
    print("=" * 120)
    print()


def print_comparison_table(comparisons: List[Tuple], file1_name: str, file2_name: str):
    """Print detailed comparison table."""
    print(f"\n{'Metric':<60} {'EMON':>15} {'PerfSpect':>15} {'% Diff':>10} {'Status':>12}")
    print("-" * 120)
    
    # Print in the order they appear in comparisons (preserves CSV order)
    for metric_name, val1, val2, pct_diff, status, symbol in comparisons:
        val1_str = f"{val1:.6g}" if val1 is not None else "N/A"
        val2_str = f"{val2:.6g}" if val2 is not None else "N/A"
        
        if pct_diff == float('inf'):
            pct_str = "INF"
        elif pct_diff is not None:
            pct_str = f"{pct_diff:+.1f}%"
        else:
            pct_str = "N/A"
        
        status_str = f"{symbol} {status}" if symbol else status
        print(f"{metric_name:<60} {val1_str:>15} {val2_str:>15} {pct_str:>10} {status_str:>12}")


def print_summary_statistics(comparisons: List[Tuple]):
    """Print summary statistics of the comparison."""
    print("\n" + "=" * 120)
    print("SUMMARY STATISTICS")
    print("=" * 120)
    
    valid_diffs = [abs(pct) for _, _, _, pct, _, _ in comparisons if pct is not None and pct != float('inf')]
    
    if not valid_diffs:
        print("No valid comparisons found.")
        return
    
    # Count by category
    excellent = sum(1 for d in valid_diffs if d < 5)
    good = sum(1 for d in valid_diffs if 5 <= d < 10)
    moderate = sum(1 for d in valid_diffs if 10 <= d < 25)
    large = sum(1 for d in valid_diffs if 25 <= d < 50)
    critical = sum(1 for d in valid_diffs if d >= 50)
    
    total = len(valid_diffs)
    
    print(f"\nTotal metrics compared: {total}")
    print(f"\nAverage absolute difference: {sum(valid_diffs) / len(valid_diffs):.2f}%")
    print(f"Median absolute difference: {sorted(valid_diffs)[len(valid_diffs)//2]:.2f}%")
    print(f"Max absolute difference: {max(valid_diffs):.2f}%")
    print(f"Min absolute difference: {min(valid_diffs):.2f}%")
    
    print("\nDistribution by category:")
    print(f"  ✓ Excellent (<5%):     {excellent:3d} ({100*excellent/total:5.1f}%)")
    print(f"  ✓ Good (5-10%):        {good:3d} ({100*good/total:5.1f}%)")
    print(f"    Moderate (10-25%):   {moderate:3d} ({100*moderate/total:5.1f}%)")
    print(f"  ⚠️ Large (25-50%):      {large:3d} ({100*large/total:5.1f}%)")
    print(f"  ⚠️ Critical (>50%):     {critical:3d} ({100*critical/total:5.1f}%)")


def print_critical_discrepancies(comparisons: List[Tuple]):
    """Print list of critical discrepancies."""
    critical = [(name, val1, val2, pct) for name, val1, val2, pct, status, _ in comparisons 
                if pct is not None and pct != float('inf') and abs(pct) >= 50]
    
    if not critical:
        print("\n✓ No critical discrepancies found (all metrics within 50%)")
        return
    
    print("\n" + "=" * 120)
    print("CRITICAL DISCREPANCIES (>50% difference)")
    print("=" * 120)
    
    for name, val1, val2, pct in sorted(critical, key=lambda x: abs(x[3]), reverse=True):
        print(f"  • {name}")
        print(f"    EMON: {val1:.6g}  |  PerfSpect: {val2:.6g}  |  Difference: {pct:+.1f}%")


def check_tma_metrics(metrics1: Dict, metrics2: Dict, file1_name: str, file2_name: str, order2: List[str]):
    """Check Top-down Microarchitecture Analysis metrics and their sum."""
    # Try different naming patterns
    tma_patterns = [
        ["Frontend_Bound(%)", "Bad_Speculation(%)", "Backend_Bound(%)", "Retiring(%)"],
        ["TMA_Frontend_Bound(%)", "TMA_Bad_Speculation(%)", "TMA_Backend_Bound(%)", "TMA_Retiring(%)"]
    ]
    
    tma_names = None
    for pattern in tma_patterns:
        if any(name in metrics1 or name in metrics2 for name in pattern):
            tma_names = pattern
            break
    
    if not tma_names:
        return
    
    # Order TMA metrics based on their appearance in file2 (summary CSV)
    tma_names_ordered = [name for name in order2 if name in tma_names]
    
    print("\n" + "=" * 120)
    print("TOP-DOWN MICROARCHITECTURE ANALYSIS (TMA)")
    print("=" * 120)
    
    sum1 = 0.0
    sum2 = 0.0
    comparisons = []
    
    # Use ordered list if available, otherwise fall back to tma_names
    names_to_process = tma_names_ordered if tma_names_ordered else tma_names
    
    for name in names_to_process:
        val1 = parse_value(metrics1.get(name, {}).get('aggregated', '')) if name in metrics1 else None
        val2 = parse_value(metrics2.get(name, {}).get('mean', '')) if name in metrics2 else None
        
        if val1 is not None:
            sum1 += val1
        if val2 is not None:
            sum2 += val2
        
        if val1 is not None and val2 is not None:
            pct_diff = calculate_percent_difference(val1, val2)
            status, symbol = categorize_difference(pct_diff)
            # Clean up metric name for display
            display_name = name.replace("TMA_", "").replace("(%)", "")
            comparisons.append((display_name, val1, val2, pct_diff, status, symbol))
    
    if comparisons:
        print(f"\n{'TMA Metric':<40} {'EMON':>15} {'PerfSpect':>15} {'% Diff':>10} {'Status':>12}")
        print("-" * 120)
        
        for name, val1, val2, pct_diff, status, symbol in comparisons:
            status_str = f"{symbol} {status}" if symbol else status
            print(f"{name:<40} {val1:>15.2f} {val2:>15.2f} {pct_diff:>+9.1f}% {status_str:>12}")
        
        print("-" * 120)
        print(f"{'Sum':<40} {sum1:>15.2f} {sum2:>15.2f}")
        
        if abs(sum1 - 100.0) > 0.1:
            print(f"\n⚠️  Warning: EMON TMA sum is {sum1:.2f}% (should be ~100%)")
        if abs(sum2 - 100.0) > 0.1:
            print(f"⚠️  Warning: PerfSpect TMA sum is {sum2:.2f}% (should be ~100%)")


def main():
    parser = argparse.ArgumentParser(
        description='Compare metric values between two CSV files',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Example:
  %(prog)s __mpp_system_view_summary.csv gnr_metrics_summary.csv
        '''
    )
    parser.add_argument('file1', help='EMON CSV file (e.g., __mpp_system_view_summary.csv)')
    parser.add_argument('file2', help='Perfspect CSV file (e.g., gnr_metrics_summary.csv)')
    
    args = parser.parse_args()
    
    try:
        metrics1, order1 = read_csv(args.file1)
        metrics2, order2 = read_csv(args.file2)
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Error reading CSV files: {e}", file=sys.stderr)
        return 1
    
    # Find common metrics
    common_metrics = set(metrics1.keys()) & set(metrics2.keys())
    
    if not common_metrics:
        print("Error: No common metrics found between the two files.", file=sys.stderr)
        return 1
    
    # Determine which columns to use for comparison
    # Look for columns containing numeric values (aggregated, mean, avg, etc.)
    sample_row1 = list(metrics1.values())[0]
    sample_row2 = list(metrics2.values())[0]
    
    # Find the value column in file1
    col1 = None
    for col_name in ['aggregated', 'mean', 'avg', 'average', 'value']:
        if col_name in sample_row1:
            col1 = col_name
            break
    if not col1:
        # Try to find any column with numeric data
        for key, val in sample_row1.items():
            if key.lower() not in ['name', 'metric', 'description', 'min', 'max', 'stddev', 'stdev', 'variation']:
                try:
                    float(val)
                    col1 = key
                    break
                except (ValueError, TypeError):
                    continue
    
    # Find the value column in file2
    col2 = None
    for col_name in ['mean', 'aggregated', 'avg', 'average', 'value']:
        if col_name in sample_row2:
            col2 = col_name
            break
    if not col2:
        # Try to find any column with numeric data
        for key, val in sample_row2.items():
            if key.lower() not in ['name', 'metric', 'description', 'min', 'max', 'stddev', 'stdev', 'variation']:
                try:
                    float(val)
                    col2 = key
                    break
                except (ValueError, TypeError):
                    continue
    
    if not col1 or not col2:
        print(f"Error: Could not determine value columns. File1 columns: {list(sample_row1.keys())}, File2 columns: {list(sample_row2.keys())}", file=sys.stderr)
        return 1
    
    # Use file2's order (typically the summary file) to preserve metric ordering
    # This ensures metrics appear in the same order as in the summary CSV
    ordered_common_metrics = [m for m in order2 if m in common_metrics]
    
    # Collect comparisons in the order they appear in file2
    comparisons = []
    for metric_name in ordered_common_metrics:
        val1 = parse_value(metrics1[metric_name].get(col1, ''))
        val2 = parse_value(metrics2[metric_name].get(col2, ''))
        
        if val1 is not None and val2 is not None:
            pct_diff = calculate_percent_difference(val1, val2)
            status, symbol = categorize_difference(pct_diff)
            comparisons.append((metric_name, val1, val2, pct_diff, status, symbol))
    
    # Print results
    print_header()
    print(f"File 1: {args.file1} (using '{col1}' column)")
    print(f"File 2: {args.file2} (using '{col2}' column)")
    print(f"Common metrics found: {len(comparisons)}")
    
    print_comparison_table(comparisons, "EMON", "PerfSpect")
    
    print_summary_statistics(comparisons)
    print_critical_discrepancies(comparisons)
    check_tma_metrics(metrics1, metrics2, "EMON", "PerfSpect", order2)
    
    print("\n" + "=" * 120)
    
    return 0


if __name__ == '__main__':
    sys.exit(main())
